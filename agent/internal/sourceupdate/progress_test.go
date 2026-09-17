package sourceupdate

import "testing"

func TestParseUpdateStep(t *testing.T) {
	cases := []struct {
		line    string
		phase   string
		detail  string
		percent int
		ok      bool
	}{
		{"==> Updating source (git pull --ff-only)", "fetching", "Fetching the new release from git", 12, true},
		{"==> Restoring source from git", "fetching", "Restoring the source tree", 10, true},
		{"==> Mode: compose", "downloading", "Preparing to build Docker images", 18, true},
		{"==> Mode: source", "building", "Preparing to rebuild from source", 18, true},
		{"==> Building images (docker compose build)", "downloading", "Downloading and building Docker images", 40, true},
		{"==> Building Go binaries", "building", "Compiling control plane, agent, and CLI", 35, true},
		{"==> Building web UI (npm install + next build)", "downloading", "Downloading packages and building the web UI", 55, true},
		{"==> Applying database migrations", "migrating", "Applying database migrations", 72, true},
		{"==> Restarting services (croncompose-ctl.sh restart)", "stopping", "Stopping the server so it can restart", 85, true},
		{"==> Starting / restarting services (docker compose up -d)", "restarting", "Starting updated containers", 88, true},
		{"==> Done", "restarting", "Restarting so the new version can come up", 92, true},
		{"  ok control-plane", "", "", 0, false},
		{"", "", "", 0, false},
		{"\x1b[1m==>\x1b[0m Building Go binaries", "building", "Compiling control plane, agent, and CLI", 35, true},
	}
	for _, tc := range cases {
		got, ok := ParseUpdateStep(tc.line)
		if ok != tc.ok {
			t.Errorf("ParseUpdateStep(%q) ok=%v, want %v", tc.line, ok, tc.ok)
			continue
		}
		if !ok {
			continue
		}
		if got.Phase != tc.phase || got.Detail != tc.detail || got.Percent != tc.percent {
			t.Errorf("ParseUpdateStep(%q) = %+v, want phase=%s detail=%q percent=%d",
				tc.line, got, tc.phase, tc.detail, tc.percent)
		}
	}
}
