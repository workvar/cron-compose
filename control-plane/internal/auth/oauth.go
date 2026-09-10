package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

const (
	oauthStateCookie   = "cc_oauth_state"
	oauthPurposeCookie = "cc_oauth_purpose"
	oauthNextCookie    = "cc_oauth_next"
	oauthCookieMaxAge  = 10 * time.Minute
)

// OAuthProvider is GitHub or GitLab OAuth app credentials.
type OAuthProvider struct {
	Name         string // github | gitlab
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	APIBase      string
}

// Enabled reports whether the provider can start an OAuth flow.
func (p OAuthProvider) Enabled() bool {
	return p.ClientID != "" && p.RedirectURL != ""
}

type oauthHandler struct {
	store       *Store
	conns       *ConnStore
	secret      []byte
	ttl         time.Duration
	github      OAuthProvider
	gitlab      OAuthProvider
	postPath    string
	http        *http.Client
	defaultRole string
}

// RegisterOAuth attaches GitHub/GitLab login and "connect git" routes.
func RegisterOAuth(r fiber.Router, store *Store, conns *ConnStore, secret []byte, github, gitlab OAuthProvider, postPath, defaultRole string) {
	if postPath == "" {
		postPath = "/"
	}
	if defaultRole == "" {
		defaultRole = "viewer"
	}
	h := &oauthHandler{
		store: store, conns: conns, secret: secret, ttl: 7 * 24 * time.Hour,
		github: github, gitlab: gitlab, postPath: postPath,
		http:        &http.Client{Timeout: 20 * time.Second},
		defaultRole: defaultRole,
	}
	r.Get("/auth/github/start", h.startGitHub)
	r.Get("/auth/github/callback", h.callbackGitHub)
	r.Get("/auth/gitlab/start", h.startGitLab)
	r.Get("/auth/gitlab/callback", h.callbackGitLab)
}

func (h *oauthHandler) startGitHub(c fiber.Ctx) error {
	return h.start(c, h.github, githubScopes(c.Query("purpose")))
}

func (h *oauthHandler) startGitLab(c fiber.Ctx) error {
	return h.start(c, h.gitlab, gitlabScopes(c.Query("purpose")))
}

func githubScopes(purpose string) string {
	if purpose == "git" {
		return "repo read:user user:email"
	}
	return "read:user user:email"
}

func gitlabScopes(purpose string) string {
	if purpose == "git" {
		return "api read_user read_repository write_repository"
	}
	return "read_user"
}

func (h *oauthHandler) start(c fiber.Ctx, p OAuthProvider, scope string) error {
	if !p.Enabled() {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fiber.Map{"code": "disabled", "message": p.Name + " oauth is not configured"},
		})
	}
	purpose := c.Query("purpose")
	if purpose != "git" {
		purpose = "login"
	}
	if purpose == "git" {
		if c.Cookies(cookieName) == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": fiber.Map{"code": "unauthenticated", "message": "sign in before connecting git"},
			})
		}
	}
	state, err := randomURLSafe(24)
	if err != nil {
		return err
	}
	setCookie(c, oauthStateCookie, state)
	setCookie(c, oauthPurposeCookie, purpose)
	if to := c.Query("next"); to != "" {
		setCookie(c, oauthNextCookie, to)
	}
	u, _ := url.Parse(p.AuthURL)
	q := u.Query()
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", p.RedirectURL)
	q.Set("state", state)
	q.Set("scope", scope)
	q.Set("response_type", "code")
	if p.Name == "gitlab" {
		q.Set("scope", strings.ReplaceAll(scope, " ", "+"))
		// GitLab wants space-separated; url.Values encodes spaces as +. Fine.
		q.Set("scope", scope)
	}
	u.RawQuery = q.Encode()
	return c.Redirect().To(u.String())
}

func (h *oauthHandler) callbackGitHub(c fiber.Ctx) error {
	return h.callback(c, h.github)
}

func (h *oauthHandler) callbackGitLab(c fiber.Ctx) error {
	return h.callback(c, h.gitlab)
}

func (h *oauthHandler) callback(c fiber.Ctx, p OAuthProvider) error {
	if errMsg := c.Query("error"); errMsg != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "oauth_error", "message": errMsg},
		})
	}
	want := c.Cookies(oauthStateCookie)
	if want == "" || want != c.Query("state") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "bad_state", "message": "state mismatch"},
		})
	}
	purpose := c.Cookies(oauthPurposeCookie)
	if purpose != "git" {
		purpose = "login"
	}
	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{"code": "missing_code", "message": "missing code"},
		})
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()

	tok, err := h.exchange(ctx, p, code)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "exchange_failed", "message": err.Error()},
		})
	}
	profile, err := h.profile(ctx, p, tok)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "profile_failed", "message": err.Error()},
		})
	}

	clearCookie(c, oauthStateCookie)
	clearCookie(c, oauthPurposeCookie)

	if purpose == "git" {
		sess, err := ParseSession(h.secret, c.Cookies(cookieName))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": fiber.Map{"code": "unauthenticated", "message": "sign in before connecting git"},
			})
		}
		if err := h.conns.Upsert(ctx, sess.UserID, p.Name, "git", profile, tok); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fiber.Map{"code": "save_failed", "message": err.Error()},
			})
		}
		to := c.Cookies(oauthNextCookie)
		clearCookie(c, oauthNextCookie)
		if to == "" || !looksSafeRedirect(to) {
			to = "/app/settings"
		}
		return c.Redirect().To(to)
	}

	u, err := h.provision(ctx, p, profile)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{"code": "provision_failed", "message": err.Error()},
		})
	}
	exp := time.Now().Add(h.ttl)
	c.Cookie(&fiber.Cookie{
		Name: cookieName, Value: SignSession(h.secret, Session{UserID: u.ID, ExpiresAt: exp}),
		Path: "/", Expires: exp, HTTPOnly: true, SameSite: "Lax",
	})
	to := c.Cookies(oauthNextCookie)
	clearCookie(c, oauthNextCookie)
	if to == "" || !looksSafeRedirect(to) {
		to = h.postPath
	}
	if !strings.HasPrefix(to, "/app") && to == "/" {
		to = "/app"
	}
	return c.Redirect().To(to)
}

type oauthProfile struct {
	ID    string
	Login string
	Email string
	Name  string
}

func (h *oauthHandler) exchange(ctx context.Context, p OAuthProvider, code string) (string, error) {
	form := url.Values{
		"client_id":     {p.ClientID},
		"client_secret": {p.ClientSecret},
		"code":          {code},
		"redirect_uri":  {p.RedirectURL},
		"grant_type":    {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	req.Header.Set("accept", "application/json")
	res, err := h.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("token http %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", errors.New("no access_token")
	}
	return out.AccessToken, nil
}

func (h *oauthHandler) profile(ctx context.Context, p OAuthProvider, token string) (oauthProfile, error) {
	if p.Name == "gitlab" {
		return h.gitlabProfile(ctx, p, token)
	}
	return h.githubProfile(ctx, p, token)
}

func (h *oauthHandler) githubProfile(ctx context.Context, p OAuthProvider, token string) (oauthProfile, error) {
	var user struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := h.getJSON(ctx, p.APIBase+"/user", token, &user); err != nil {
		return oauthProfile{}, err
	}
	email := user.Email
	if email == "" {
		var emails []struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		_ = h.getJSON(ctx, p.APIBase+"/user/emails", token, &emails)
		for _, e := range emails {
			if e.Primary && e.Verified {
				email = e.Email
				break
			}
		}
		if email == "" && len(emails) > 0 {
			email = emails[0].Email
		}
	}
	if email == "" {
		email = user.Login + "@users.noreply.github.com"
	}
	name := user.Name
	if name == "" {
		name = user.Login
	}
	return oauthProfile{ID: fmt.Sprintf("%d", user.ID), Login: user.Login, Email: email, Name: name}, nil
}

func (h *oauthHandler) gitlabProfile(ctx context.Context, p OAuthProvider, token string) (oauthProfile, error) {
	var user struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
		Email    string `json:"email"`
	}
	if err := h.getJSON(ctx, p.APIBase+"/user", token, &user); err != nil {
		return oauthProfile{}, err
	}
	email := user.Email
	if email == "" {
		email = user.Username + "@users.noreply.gitlab.com"
	}
	name := user.Name
	if name == "" {
		name = user.Username
	}
	return oauthProfile{ID: fmt.Sprintf("%d", user.ID), Login: user.Username, Email: email, Name: name}, nil
}

func (h *oauthHandler) getJSON(ctx context.Context, rawURL, token string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("authorization", "Bearer "+token)
	req.Header.Set("accept", "application/json")
	res, err := h.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("http %d: %s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(res.Body).Decode(dest)
}

func (h *oauthHandler) provision(ctx context.Context, p OAuthProvider, profile oauthProfile) (User, error) {
	if uid, err := h.conns.UserForIdentity(ctx, p.Name, profile.ID); err == nil {
		return h.store.GetByID(ctx, uid)
	}
	u, _, err := h.store.GetByEmailWithHash(ctx, profile.Email)
	if err == nil {
		_ = h.conns.LinkIdentity(ctx, u.ID, p.Name, profile)
		return u, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return User{}, err
	}
	u, err = h.store.Upsert(ctx, profile.Email, profile.Name, h.defaultRole, "")
	if err != nil {
		return User{}, err
	}
	_ = h.conns.LinkIdentity(ctx, u.ID, p.Name, profile)
	return u, nil
}

func setCookie(c fiber.Ctx, name, value string) {
	c.Cookie(&fiber.Cookie{
		Name: name, Value: value, Path: "/",
		Expires: time.Now().Add(oauthCookieMaxAge), HTTPOnly: true, SameSite: "Lax",
	})
}

func clearCookie(c fiber.Ctx, name string) {
	c.Cookie(&fiber.Cookie{Name: name, Value: "", Path: "/", Expires: time.Unix(0, 0), HTTPOnly: true})
}

// GitHubProvider returns the GitHub OAuth app config.
func GitHubProvider(clientID, secret, redirect string) OAuthProvider {
	return OAuthProvider{
		Name: "github", ClientID: clientID, ClientSecret: secret, RedirectURL: redirect,
		AuthURL:  "https://github.com/login/oauth/authorize",
		TokenURL: "https://github.com/login/oauth/access_token",
		APIBase:  "https://api.github.com",
	}
}

// GitLabProvider returns GitLab (or self-hosted) OAuth app config.
func GitLabProvider(clientID, secret, redirect, base string) OAuthProvider {
	base = strings.TrimRight(base, "/")
	if base == "" {
		base = "https://gitlab.com"
	}
	return OAuthProvider{
		Name: "gitlab", ClientID: clientID, ClientSecret: secret, RedirectURL: redirect,
		AuthURL:  base + "/oauth/authorize",
		TokenURL: base + "/oauth/token",
		APIBase:  base + "/api/v4",
	}
}
