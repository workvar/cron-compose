package servers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

// Production change that would fail this test: allowing POST /servers/:id/agent-root
// when the actor has zero passkeys (200 instead of 403 passkey_required).
func TestAgentRootRequiresPasskey(t *testing.T) {
	h := &handler{
		passkeys: stubPasskeys{has: false},
		stepUp:   stubPasskeys{},
		roots:    &fakeRootStore{srv: Server{ID: "srv-1"}},
		audit:    &recordingAudit{},
	}
	app := newAgentRootApp(t, h, "user-1", "admin")

	resp := postAgentRoot(t, app, "srv-1", `{"enabled":true,"credential":{}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d want 403 body=%s", resp.StatusCode, body)
	}
	if got := errorCode(t, resp); got != "passkey_required" {
		t.Fatalf("code=%q want passkey_required", got)
	}
}

// Production change that would fail this test: treating a failed WebAuthn
// assertion as success (200) instead of 403 invalid_assertion.
func TestAgentRootRejectsBadAssertion(t *testing.T) {
	h := &handler{
		passkeys: stubPasskeys{has: true},
		stepUp:   stubPasskeys{stepErr: errors.New("invalid assertion")},
		roots:    &fakeRootStore{srv: Server{ID: "srv-1"}},
		audit:    &recordingAudit{},
	}
	app := newAgentRootApp(t, h, "user-1", "admin")

	resp := postAgentRoot(t, app, "srv-1", `{"enabled":true,"challenge_id":"ch-1","credential":{"id":"bad"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d want 403 body=%s", resp.StatusCode, body)
	}
	if got := errorCode(t, resp); got != "invalid_assertion" {
		t.Fatalf("code=%q want invalid_assertion", got)
	}
	if h.roots.(*fakeRootStore).setCalls != 0 {
		t.Fatal("store must not be updated on a bad assertion")
	}
}

// Production change that would fail this test: skipping the DB flag / audit
// after a valid step-up, or not pushing AgentRootCommand after the persist.
func TestAgentRootToggleSuccess(t *testing.T) {
	roots := &fakeRootStore{srv: Server{ID: "srv-1", Name: "pi"}}
	auditLog := &recordingAudit{}
	sender := &stubRootSender{}
	h := &handler{
		passkeys: stubPasskeys{has: true},
		stepUp:   stubPasskeys{},
		roots:    roots,
		audit:    auditLog,
		rootCmd:  sender,
	}
	app := newAgentRootApp(t, h, "user-1", "admin")

	resp := postAgentRoot(t, app, "srv-1", `{"enabled":true,"challenge_id":"ch-1","credential":{"id":"ok"}}`)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", resp.StatusCode, body)
	}
	if roots.setCalls != 1 || roots.enabled == nil || !*roots.enabled || roots.by != "user-1" {
		t.Fatalf("setCalls=%d enabled=%v by=%q", roots.setCalls, roots.enabled, roots.by)
	}
	if len(auditLog.actions) != 1 || auditLog.actions[0] != "server.agent_root.enable" {
		t.Fatalf("audit=%v want [server.agent_root.enable]", auditLog.actions)
	}
	var got Server
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json: %v body=%s", err, body)
	}
	if !got.AgentRootEnabled || got.ID != "srv-1" {
		t.Fatalf("got %+v", got)
	}
	if sender.calls != 1 || sender.serverID != "srv-1" || !sender.enabled {
		t.Fatalf("send calls=%d id=%q enabled=%v", sender.calls, sender.serverID, sender.enabled)
	}
}

func TestAgentRootDisableAuditsDisable(t *testing.T) {
	roots := &fakeRootStore{srv: Server{ID: "srv-1"}}
	auditLog := &recordingAudit{}
	sender := &stubRootSender{}
	h := &handler{
		passkeys: stubPasskeys{has: true},
		stepUp:   stubPasskeys{},
		roots:    roots,
		audit:    auditLog,
		rootCmd:  sender,
	}
	app := newAgentRootApp(t, h, "user-1", "admin")

	resp := postAgentRoot(t, app, "srv-1", `{"enabled":false,"challenge_id":"ch-1","credential":{"id":"ok"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if roots.enabled == nil || *roots.enabled {
		t.Fatal("expected enabled=false")
	}
	if len(auditLog.actions) != 1 || auditLog.actions[0] != "server.agent_root.disable" {
		t.Fatalf("audit=%v", auditLog.actions)
	}
	if sender.calls != 1 || sender.enabled {
		t.Fatalf("send calls=%d enabled=%v want demote", sender.calls, sender.enabled)
	}
}

// Production change that would fail this test: treating a disconnected agent as
// success (200) instead of matching SendAgentUpdate's agent_offline error.
func TestAgentRootOfflineAfterPersist(t *testing.T) {
	roots := &fakeRootStore{srv: Server{ID: "srv-1"}}
	h := &handler{
		passkeys: stubPasskeys{has: true},
		stepUp:   stubPasskeys{},
		roots:    roots,
		audit:    &recordingAudit{},
		rootCmd:  &stubRootSender{err: agentgw.ErrAgentOffline},
	}
	app := newAgentRootApp(t, h, "user-1", "admin")

	resp := postAgentRoot(t, app, "srv-1", `{"enabled":true,"challenge_id":"ch-1","credential":{"id":"ok"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d want 503 body=%s", resp.StatusCode, body)
	}
	if got := errorCode(t, resp); got != "agent_offline" {
		t.Fatalf("code=%q want agent_offline", got)
	}
	if roots.setCalls != 1 {
		t.Fatal("flag must be persisted even when the agent is offline")
	}
}

// Production change that would fail this test: ignoring the cc_webauthn cookie
// when challenge_id is omitted from the body (Task 2 pattern).
func TestAgentRootReadsChallengeIDFromCookie(t *testing.T) {
	step := &captureStepUp{}
	h := &handler{
		passkeys: stubPasskeys{has: true},
		stepUp:   step,
		roots:    &fakeRootStore{srv: Server{ID: "srv-1"}},
		audit:    &recordingAudit{},
	}
	app := newAgentRootApp(t, h, "user-1", "admin")
	req := httptest.NewRequest(http.MethodPost, "/servers/srv-1/agent-root", bytes.NewReader([]byte(`{"enabled":true,"credential":{"id":"ok"}}`)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.ChallengeCookie, Value: "cookie-ch"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if step.challengeID != "cookie-ch" {
		t.Fatalf("challenge_id=%q want cookie-ch", step.challengeID)
	}
}

func TestAgentRootForbiddenForOperator(t *testing.T) {
	roots := &fakeRootStore{srv: Server{ID: "srv-1"}}
	h := &handler{
		passkeys: stubPasskeys{has: true},
		stepUp:   stubPasskeys{},
		roots:    roots,
		audit:    &recordingAudit{},
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		auth.AttachIdentity(c, "user-1", "operator")
		return c.Next()
	})
	app.Post("/servers/:id/agent-root", auth.RequireRole("admin"), h.setAgentRoot)

	resp := postAgentRoot(t, app, "srv-1", `{"enabled":true,"challenge_id":"ch-1","credential":{"id":"ok"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d want 403 body=%s", resp.StatusCode, body)
	}
	if got := errorCode(t, resp); got != "forbidden" {
		t.Fatalf("code=%q want forbidden", got)
	}
	if roots.setCalls != 0 {
		t.Fatal("operator must not toggle agent root")
	}
}

type captureStepUp struct {
	challengeID string
}

func (c *captureStepUp) VerifyStepUp(_ context.Context, _, challengeID string, _ []byte) error {
	c.challengeID = challengeID
	return nil
}

type stubPasskeys struct {
	has     bool
	hasErr  error
	stepErr error
}

func (s stubPasskeys) HasPasskey(context.Context, string) (bool, error) {
	return s.has, s.hasErr
}

func (s stubPasskeys) VerifyStepUp(context.Context, string, string, []byte) error {
	return s.stepErr
}

type stubRootSender struct {
	serverID string
	enabled  bool
	err      error
	calls    int
}

func (s *stubRootSender) SendAgentRootCommand(serverID string, enabled bool) error {
	s.calls++
	s.serverID = serverID
	s.enabled = enabled
	return s.err
}

type fakeRootStore struct {
	srv      Server
	getErr   error
	setErr   error
	setCalls int
	enabled  *bool
	by       string
}

func (f *fakeRootStore) SetAgentRootEnabled(_ context.Context, id string, enabled bool, changedBy string) (Server, error) {
	if f.setErr != nil {
		return Server{}, f.setErr
	}
	f.setCalls++
	f.enabled = &enabled
	f.by = changedBy
	f.srv.ID = id
	f.srv.AgentRootEnabled = enabled
	f.srv.AgentRootChangedBy = &changedBy
	return f.srv, nil
}

type recordingAudit struct {
	actions []string
}

func (r *recordingAudit) Write(_ context.Context, _, action, _, _ string, _ map[string]any) {
	r.actions = append(r.actions, action)
}

func newAgentRootApp(t *testing.T, h *handler, userID, role string) *fiber.App {
	t.Helper()
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		auth.AttachIdentity(c, userID, role)
		return c.Next()
	})
	app.Post("/servers/:id/agent-root", h.setAgentRoot)
	return app
}

func postAgentRoot(t *testing.T, app *fiber.App, serverID, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/agent-root", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func errorCode(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json: %v body=%s", err, body)
	}
	return got.Error.Code
}
