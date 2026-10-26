package autoupdate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		cur, lat string
		want     bool
	}{
		{"0.0.63", "v0.0.64", true},
		{"v0.0.64", "v0.0.64", false},
		{"0.1.0", "v0.0.99", false},
		{"0.1.0-dev", "v0.1.0", false},
		{"0.1.0-dev", "v0.1.1", true},
		{"1.2", "1.2.1", true},
	}
	for _, c := range cases {
		if got := Newer(c.cur, c.lat); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.cur, c.lat, got, c.want)
		}
	}
}

func TestLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/cc/releases/latest" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9"}`))
	}))
	defer srv.Close()
	old := apiBase
	apiBase = srv.URL
	defer func() { apiBase = old }()

	tag, err := Latest(context.Background(), "acme/cc")
	if err != nil || tag != "v9.9.9" {
		t.Fatalf("Latest = %q, %v", tag, err)
	}
	if _, err := Latest(context.Background(), "acme/missing"); err == nil {
		t.Fatal("expected an error for a missing repo")
	}
}
