package deploys

import "testing"

func TestGitHubWebhookValid(t *testing.T) {
	secret := "whsec"
	body := []byte(`{"ref":"refs/heads/main"}`)
	sig := GitHubSignature(secret, body)
	if !ValidGitHubSignature(secret, sig, body) {
		t.Fatal("expected valid signature")
	}
	if ValidGitHubSignature(secret, "sha256=deadbeef", body) {
		t.Fatal("tampered signature must fail")
	}
}

func TestGitLabToken(t *testing.T) {
	if !ValidGitLabToken("abc", "abc") {
		t.Fatal("matching token should pass")
	}
	if ValidGitLabToken("abc", "nope") {
		t.Fatal("mismatch should fail")
	}
}

func TestPushBranch(t *testing.T) {
	if got := BranchFromRef("refs/heads/main"); got != "main" {
		t.Errorf("got %q", got)
	}
	if got := BranchFromRef("refs/tags/v1"); got != "" {
		t.Errorf("tags should be ignored, got %q", got)
	}
}
