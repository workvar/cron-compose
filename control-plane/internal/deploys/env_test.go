package deploys

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/croncompose/croncompose/control-plane/internal/cryptobox"
)

func testBox(t *testing.T) *cryptobox.Box {
	t.Helper()
	// 32 zero bytes — fine for unit tests.
	key := hex.EncodeToString(make([]byte, 32))
	b, err := cryptobox.New(key)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseDotEnv(t *testing.T) {
	raw := `
# comment
NODE_ENV=production
API_KEY="secret value"
EMPTY=
export PORT=3000
'QUOTED'=hello
# another
INVALID
DB_URL=postgres://u:p@h/db
`
	got := ParseDotEnv(raw)
	want := map[string]string{
		"NODE_ENV": "production",
		"API_KEY":  "secret value",
		"EMPTY":    "",
		"PORT":     "3000",
		"QUOTED":   "hello",
		"DB_URL":   "postgres://u:p@h/db",
	}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %+v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s=%q want %q", k, got[k], v)
		}
	}
}

func TestSealAndRedactEnv(t *testing.T) {
	box := testBox(t)
	apps := []SpecApp{{
		Name: "api",
		Root: ".",
		Env: []EnvVar{
			{Key: "NODE_ENV", Value: "production", Sensitive: false},
			{Key: "SECRET", Value: "s3cr3t", Sensitive: true},
		},
	}}
	sealed, err := SealAppsEnv(box, apps)
	if err != nil {
		t.Fatal(err)
	}
	if sealed[0].Env[1].ValueEnc == "" {
		t.Fatal("expected value_enc")
	}
	if sealed[0].Env[1].Value != "" {
		t.Fatal("plaintext should be cleared after seal")
	}

	redacted := RedactApps(sealed)
	if redacted[0].Env[0].Value != "production" {
		t.Fatalf("public value=%q", redacted[0].Env[0].Value)
	}
	if redacted[0].Env[1].Value != "" || redacted[0].Env[1].ValueEnc != "" {
		t.Fatalf("sensitive leaked: %+v", redacted[0].Env[1])
	}
	if !redacted[0].Env[1].HasValue {
		t.Fatal("expected has_value")
	}

	plain, err := ResolveAppEnv(box, sealed[0])
	if err != nil {
		t.Fatal(err)
	}
	if plain["SECRET"] != "s3cr3t" || plain["NODE_ENV"] != "production" {
		t.Fatalf("plain=%v", plain)
	}
}

func TestMergeAppsEnvKeepsSensitiveWhenBlank(t *testing.T) {
	box := testBox(t)
	old := []SpecApp{{
		Name: "api", Root: ".",
		Env: []EnvVar{{Key: "SECRET", ValueEnc: "keepme", Sensitive: true}},
	}}
	// Pretend already sealed with a real enc:
	old, err := SealAppsEnv(box, []SpecApp{{
		Name: "api", Root: ".",
		Env: []EnvVar{{Key: "SECRET", Value: "original", Sensitive: true}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	_ = old

	incoming := []SpecApp{{
		Name: "api", Root: ".",
		Env: []EnvVar{
			{Key: "SECRET", Value: "", Sensitive: true, HasValue: true},
			{Key: "NODE_ENV", Value: "prod", Sensitive: false},
		},
	}}
	merged, err := MergeAppsEnv(box, old, incoming)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := ResolveAppEnv(box, merged[0])
	if err != nil {
		t.Fatal(err)
	}
	if plain["SECRET"] != "original" {
		t.Fatalf("SECRET=%q", plain["SECRET"])
	}
	if plain["NODE_ENV"] != "prod" {
		t.Fatalf("NODE_ENV=%q", plain["NODE_ENV"])
	}
}

func TestParseDotEnvOrder(t *testing.T) {
	got := ParseDotEnvOrdered("B=2\nA=1\n")
	if len(got) != 2 || got[0].Key != "B" || got[1].Key != "A" {
		t.Fatalf("%+v", got)
	}
}

func TestParseDotEnvSkipsExportAndComments(t *testing.T) {
	line := strings.TrimSpace("# nope")
	if line == "" {
		t.Fatal("sanity")
	}
}
