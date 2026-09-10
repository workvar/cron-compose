package deploys

import "testing"

func TestAuthenticatedCloneURL(t *testing.T) {
	got := AuthenticatedCloneURL("https://github.com/acme/web.git", "github", "tok")
	if got != "https://x-access-token:tok@github.com/acme/web.git" {
		t.Errorf("got %q", got)
	}
	if PublicCloneURL(got) != "https://github.com/acme/web.git" {
		t.Errorf("public = %q", PublicCloneURL(got))
	}
}
