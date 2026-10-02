package enroll

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Production change that would fail this test: renaming or dropping run_as_root, which
// the control plane reads to learn whether the agent was installed to run as root.
func TestPostSendsRunAsRoot(t *testing.T) {
	for _, runAsRoot := range []bool{true, false} {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/agents/enroll" {
				t.Errorf("path=%q want /agents/enroll", r.URL.Path)
			}
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &got); err != nil {
				t.Errorf("body is not JSON: %v", err)
			}
			_, _ = w.Write([]byte(`{"server_id":"srv-1","client_cert_pem":"c","server_ca_pem":"ca"}`))
		}))

		_, err := Post(srv.URL, Request{Token: "t", RunAsRoot: runAsRoot})
		srv.Close()
		if err != nil {
			t.Fatalf("runAsRoot=%v: %v", runAsRoot, err)
		}
		v, present := got["run_as_root"]
		if !present {
			t.Fatalf("runAsRoot=%v: run_as_root missing from the request body: %v", runAsRoot, got)
		}
		if v != runAsRoot {
			t.Fatalf("run_as_root=%v want %v", v, runAsRoot)
		}
	}
}

// Production change that would fail this test: dropping agent_secret or grpc_mode from
// the response, so an edge-mode agent would have no secret to log in with.
func TestPostReadsSecretAndMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"server_id":"srv-1","client_cert_pem":"c","server_ca_pem":"ca","agent_secret":"sek","grpc_mode":"edge"}`))
	}))
	defer srv.Close()
	resp, err := Post(srv.URL, Request{Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.AgentSecret != "sek" || resp.GRPCMode != "edge" {
		t.Fatalf("got %+v", resp)
	}
}
