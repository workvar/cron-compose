package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestParseOAuthTokenResponse_AccessOnly(t *testing.T) {
	tok, err := parseOAuthTokenResponse([]byte(`{"access_token":"a1","token_type":"bearer","scope":"repo"}`))
	if err != nil {
		t.Fatal(err)
	}
	if tok.Access != "a1" || tok.Refresh != "" || !tok.ExpiresAt.IsZero() {
		t.Fatalf("%+v", tok)
	}
}

func TestParseOAuthTokenResponse_WithRefreshAndExpiry(t *testing.T) {
	before := time.Now()
	tok, err := parseOAuthTokenResponse([]byte(`{
		"access_token":"a2",
		"refresh_token":"r2",
		"expires_in":3600,
		"token_type":"bearer"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if tok.Access != "a2" || tok.Refresh != "r2" {
		t.Fatalf("%+v", tok)
	}
	if tok.ExpiresAt.Before(before.Add(50*time.Minute)) || tok.ExpiresAt.After(before.Add(70*time.Minute)) {
		t.Fatalf("expires_at=%v", tok.ExpiresAt)
	}
}

func TestParseOAuthTokenResponse_ErrorField(t *testing.T) {
	_, err := parseOAuthTokenResponse([]byte(`{"error":"bad_verification_code","error_description":"expired"}`))
	if err == nil || !strings.Contains(err.Error(), "bad_verification_code") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseOAuthTokenResponse_MissingAccess(t *testing.T) {
	_, err := parseOAuthTokenResponse([]byte(`{"refresh_token":"r"}`))
	if err == nil || !strings.Contains(err.Error(), "no access_token") {
		t.Fatalf("err=%v", err)
	}
}

func TestNeedsRefresh(t *testing.T) {
	s := &ConnStore{resolve: func(context.Context, string) (OAuthProvider, error) {
		return GitHubProvider("id", "sec", "https://example/cb"), nil
	}}
	cases := []struct {
		name string
		tok  storedGitToken
		want bool
	}{
		{name: "no refresh", tok: storedGitToken{Access: "a", ExpiresAt: time.Now()}, want: false},
		{name: "no expiry", tok: storedGitToken{Access: "a", Refresh: "r"}, want: false},
		{name: "fresh", tok: storedGitToken{Access: "a", Refresh: "r", ExpiresAt: time.Now().Add(time.Hour)}, want: false},
		{name: "within skew", tok: storedGitToken{Access: "a", Refresh: "r", ExpiresAt: time.Now().Add(time.Minute)}, want: true},
		{name: "already expired", tok: storedGitToken{Access: "a", Refresh: "r", ExpiresAt: time.Now().Add(-time.Minute)}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := s.needsRefresh(tc.tok); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
	t.Run("no resolver", func(t *testing.T) {
		bare := &ConnStore{}
		tok := storedGitToken{Access: "a", Refresh: "r", ExpiresAt: time.Now().Add(-time.Minute)}
		if bare.needsRefresh(tok) {
			t.Fatal("expected false without resolver")
		}
	})
}

func TestRefreshGitToken(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotForm, _ = url.ParseQuery(string(body))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    7200,
			"token_type":    "bearer",
		})
	}))
	defer srv.Close()

	s := &ConnStore{
		http: srv.Client(),
		resolve: func(context.Context, string) (OAuthProvider, error) {
			return OAuthProvider{
				Name: "github", ClientID: "cid", ClientSecret: "csec",
				RedirectURL: "https://app/cb", TokenURL: srv.URL,
			}, nil
		},
	}
	tok, err := s.refreshGitToken(context.Background(), "github", "old-refresh")
	if err != nil {
		t.Fatal(err)
	}
	if tok.Access != "new-access" || tok.Refresh != "new-refresh" || tok.ExpiresAt.IsZero() {
		t.Fatalf("%+v", tok)
	}
	if gotForm.Get("grant_type") != "refresh_token" ||
		gotForm.Get("refresh_token") != "old-refresh" ||
		gotForm.Get("client_id") != "cid" ||
		gotForm.Get("client_secret") != "csec" {
		t.Fatalf("form=%v", gotForm)
	}
}

func TestRefreshGitToken_KeepsPriorRefreshWhenOmitted(t *testing.T) {
	// Mirrors Token()'s fallback: some providers omit refresh_token on renewal.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "only-access",
			"expires_in":   60,
		})
	}))
	defer srv.Close()
	s := &ConnStore{
		http: srv.Client(),
		resolve: func(context.Context, string) (OAuthProvider, error) {
			return OAuthProvider{
				Name: "gitlab", ClientID: "id", ClientSecret: "sec",
				RedirectURL: "https://app/cb", TokenURL: srv.URL,
			}, nil
		},
	}
	tok, err := s.refreshGitToken(context.Background(), "gitlab", "keep-me")
	if err != nil {
		t.Fatal(err)
	}
	if tok.Access != "only-access" || tok.Refresh != "" {
		t.Fatalf("%+v", tok)
	}
}
