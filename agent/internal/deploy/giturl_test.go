package deploy

import "testing"

func TestAuthenticatedCloneURL(t *testing.T) {
	got := AuthenticatedCloneURL("https://github.com/acme/web.git", "github", "tok")
	if got != "https://x-access-token:tok@github.com/acme/web.git" {
		t.Errorf("got %q", got)
	}
	if PublicCloneURL(got) != "https://github.com/acme/web.git" {
		t.Errorf("public = %q", PublicCloneURL(got))
	}
	gl := AuthenticatedCloneURL("https://gitlab.com/acme/web.git", "gitlab", "tok")
	if gl != "https://oauth2:tok@gitlab.com/acme/web.git" {
		t.Errorf("gitlab = %q", gl)
	}
}

func TestRedact(t *testing.T) {
	if got := redact("fatal: https://x-access-token:sekrit@github.com", "sekrit"); got != "fatal: https://x-access-token:***@github.com" {
		t.Errorf("got %q", got)
	}
}
