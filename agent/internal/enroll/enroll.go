// Package enroll wraps the REST call the agent makes once to exchange a one-time
// token for a signed client certificate.
package enroll

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Request mirrors the control-plane's expected body.
type Request struct {
	Token        string `json:"token"`
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	AgentVersion string `json:"agent_version"`
	CSRPEM       string `json:"csr_pem"`
	// RunAsRoot is true when the enrolling process is root, which is how the installer
	// tells the control plane whether the agent was installed to run as root.
	RunAsRoot bool `json:"run_as_root"`
}

// Response is returned by the control plane.
type Response struct {
	ServerID             string `json:"server_id"`
	ClientCertPEM        string `json:"client_cert_pem"`
	ServerCAPEM          string `json:"server_ca_pem"`
	ControlPlaneGRPCAddr string `json:"control_plane_grpc_addr"`
	AgentSecret          string `json:"agent_secret"`
	GRPCMode             string `json:"grpc_mode"`
}

// Post calls POST <baseURL>/agents/enroll. baseURL is e.g. http://localhost:8080/api/v1.
func Post(baseURL string, req Request) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(baseURL, "/") + "/agents/enroll"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("enroll: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var out Response
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	if out.ServerID == "" || out.ClientCertPEM == "" || out.ServerCAPEM == "" {
		return nil, errors.New("enroll: incomplete response")
	}
	return &out, nil
}
