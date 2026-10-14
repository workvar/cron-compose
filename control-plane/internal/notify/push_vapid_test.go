package notify

import (
	"strings"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func TestGenerateVAPIDKeysRoundTrip(t *testing.T) {
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	if pub == "" || priv == "" {
		t.Fatal("empty keys")
	}
	if strings.Contains(pub, "+") || strings.Contains(pub, "/") {
		t.Fatalf("public key should be raw URL-safe base64: %q", pub)
	}
}

func TestVapidSubscriberStripsMailto(t *testing.T) {
	if got := vapidSubscriber("mailto:ops@example.com"); got != "ops@example.com" {
		t.Fatalf("got %q", got)
	}
	if got := vapidSubscriber(""); got != "croncompose@localhost" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizePushEvents(t *testing.T) {
	got := normalizePushEvents([]string{"deploy_failed", "nope", "deploy_failed", "job_failed"})
	if len(got) != 2 || got[0] != "deploy_failed" || got[1] != "job_failed" {
		t.Fatalf("%v", got)
	}
	def := normalizePushEvents(nil)
	if len(def) != len(DefaultPushEvents) {
		t.Fatalf("defaults: %v", def)
	}
}
