package connectors

import "testing"

func TestParseGitRemote(t *testing.T) {
	cases := []struct {
		in, provider, full string
	}{
		{"git@github.com:acme/web.git", "github", "acme/web"},
		{"https://github.com/acme/web.git", "github", "acme/web"},
		{"https://gitlab.com/group/proj.git", "gitlab", "group/proj"},
		{"ssh://git@github.com/acme/web.git", "github", "acme/web"},
	}
	for _, tc := range cases {
		p, f := parseGitRemote(tc.in)
		if p != tc.provider || f != tc.full {
			t.Fatalf("%q: got %q/%q want %q/%q", tc.in, p, f, tc.provider, tc.full)
		}
	}
}

func TestStripSystemdExecStart(t *testing.T) {
	raw := "{ path=/usr/bin/node ; argv[]=/usr/bin/node /app/server.js ; ignore_errors=no ; start_time=[n/a] }"
	got := stripSystemdExecStart(raw)
	want := "/usr/bin/node /app/server.js"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestParseSystemdShowBlocks(t *testing.T) {
	raw := `Id=api.service
FragmentPath=/etc/systemd/system/api.service
ExecStart={ path=/usr/bin/node ; argv[]=/usr/bin/node server.js ; }
WorkingDirectory=/opt/api
Environment=NODE_ENV=production PORT=3000

Id=other.service
FragmentPath=/lib/systemd/system/other.service
ExecStart={ path=/bin/true ; argv[]=/bin/true ; }
WorkingDirectory=/
Environment=
`
	blocks := parseSystemdShowBlocks(raw)
	api := blocks["api.service"]
	if api == nil {
		t.Fatal("missing api.service")
	}
	if api["command"] != "/usr/bin/node server.js" {
		t.Fatalf("command: %q", api["command"])
	}
	if api["cwd"] != "/opt/api" {
		t.Fatalf("cwd: %q", api["cwd"])
	}
	if api["env_count"] != "2" {
		t.Fatalf("env_count: %q", api["env_count"])
	}
}

func TestEnvKeysCSV(t *testing.T) {
	got := envKeysCSV(map[string]string{"Z": "1", "A": "2"})
	if got != "A,Z" {
		t.Fatalf("got %q", got)
	}
}
