package deploys

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

// GitHubSignature returns the X-Hub-Signature-256 value for body.
func GitHubSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// ValidGitHubSignature checks GitHub's X-Hub-Signature-256 header.
func ValidGitHubSignature(secret, header string, body []byte) bool {
	want := GitHubSignature(secret, body)
	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(header)), []byte(want)) == 1
}

// ValidGitLabToken checks GitLab's X-Gitlab-Token header.
func ValidGitLabToken(secret, header string) bool {
	if secret == "" || header == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(secret), []byte(header)) == 1
}

// BranchFromRef extracts a branch from a git ref. Tags return empty.
func BranchFromRef(ref string) string {
	const prefix = "refs/heads/"
	if strings.HasPrefix(ref, prefix) {
		return strings.TrimPrefix(ref, prefix)
	}
	return ""
}
