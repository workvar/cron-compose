package deploys

import (
	"strings"
	"testing"
)

func issueWith(res SpecResult, level, substr string) bool {
	for _, is := range res.Issues {
		if is.Level == level && strings.Contains(is.Message, substr) {
			return true
		}
	}
	return false
}

func TestParseSpecFileFull(t *testing.T) {
	res := ParseSpecFile([]byte(`
version: 1
name: shop
repo: https://github.com/acme/shop.git
branch: main
server: pi-home
clone_path: /opt/apps/node/shop
process_manager: pm2
env:
  NODE_ENV: production
health:
  path: /healthz
  port: 3000
  timeout: 60
deploy_timeout: 900
auto_rollback: true
apps:
  - name: web
    root: apps/web
    language: node
    install: pnpm install && pnpm build
    port: 3000
    env:
      PUBLIC_URL: https://shop.example.com
  - root: ./apps/api/
    install: go build -o api .
    language: go
    process_manager: systemd
    env:
      - key: LOG_LEVEL
        value: info
`))
	if !res.Valid {
		t.Fatalf("expected valid, issues = %+v", res.Issues)
	}
	s := res.Spec
	if s.Provider != "github" || s.Repo != "acme/shop" {
		t.Errorf("repo = %s %s", s.Provider, s.Repo)
	}
	if s.Server != "pi-home" || !s.AutoRollback || s.DeployTimeout != 900 {
		t.Errorf("extras = %+v", s)
	}
	if s.Health == nil || s.Health.Path != "/healthz" || s.Health.Timeout != 60 {
		t.Errorf("health = %+v", s.Health)
	}
	if len(s.Apps) != 2 {
		t.Fatalf("apps = %+v", s.Apps)
	}
	if s.Apps[1].Name != "api" || s.Apps[1].Root != "apps/api" {
		t.Errorf("second app = %+v", s.Apps[1])
	}
	if len(s.Apps[0].Env) != 1 || s.Apps[0].Env[0].Key != "PUBLIC_URL" {
		t.Errorf("map env = %+v", s.Apps[0].Env)
	}
	if len(s.Apps[1].Env) != 1 || s.Apps[1].Env[0].Value != "info" {
		t.Errorf("list env = %+v", s.Apps[1].Env)
	}
}

func TestParseSpecFileRoundTripsWrittenSpec(t *testing.T) {
	raw, err := MarshalSpec(SpecForProject(Project{
		Name: "web", Provider: "gitlab", RepoFullName: "grp/sub/web", DefaultBranch: "main",
		Language: "node", InstallScript: "npm ci", RootDirectory: ".", ProcessManager: "pm2",
		Apps: []SpecApp{{Name: "web", Root: ".", Env: []EnvVar{{Key: "A", Value: "1"}}}},
		HealthPath: "/up", HealthPort: 8080,
	}))
	if err != nil {
		t.Fatal(err)
	}
	res := ParseSpecFile(raw)
	if !res.Valid {
		t.Fatalf("written spec should re-read cleanly: %+v\n%s", res.Issues, raw)
	}
	if res.Spec.Repo != "grp/sub/web" || res.Spec.Provider != "gitlab" || res.Spec.Health.Port != 8080 {
		t.Errorf("round trip = %+v", res.Spec)
	}
}

func TestParseSpecFileErrors(t *testing.T) {
	res := ParseSpecFile([]byte(`
version: 2
process_manager: forever
port: 70000
clone_path: relative/path
instal: npm ci
health:
  path: healthz
env:
  1BAD: x
  API_TOKEN: abc
apps:
  - root: ../outside
  - name: a
    root: web
  - name: a
    root: web
`))
	if res.Valid {
		t.Fatal("expected invalid")
	}
	for _, want := range []struct{ level, msg string }{
		{"error", "newer than this CronCompose"},
		{"error", "process_manager must be one of"},
		{"error", "port must be between"},
		{"error", "absolute path"},
		{"warning", `unknown key "instal" on line 6`},
		{"error", "health.path must start with /"},
		{"error", `env key "1BAD"`},
		{"warning", "API_TOKEN looks like a secret"},
		{"error", "points outside the repo"},
		{"error", `two apps are named "a"`},
		{"error", `two apps use root "web"`},
	} {
		if !issueWith(res, want.level, want.msg) {
			t.Errorf("missing %s %q in %+v", want.level, want.msg, res.Issues)
		}
	}
}

func TestParseSpecFileBadYAML(t *testing.T) {
	res := ParseSpecFile([]byte("name: [unclosed"))
	if res.Valid || !issueWith(res, "error", "not valid YAML") {
		t.Fatalf("issues = %+v", res.Issues)
	}
	if res := ParseSpecFile([]byte("   ")); res.Valid {
		t.Fatal("empty file should be invalid")
	}
}

func TestParseRepoRef(t *testing.T) {
	cases := []struct{ in, provider, full string }{
		{"acme/web", "", "acme/web"},
		{"https://github.com/acme/web", "github", "acme/web"},
		{"https://github.com/acme/web.git", "github", "acme/web"},
		{"https://github.com/acme/web/tree/main/apps", "github", "acme/web"},
		{"git@github.com:acme/web.git", "github", "acme/web"},
		{"https://gitlab.com/grp/sub/proj", "gitlab", "grp/sub/proj"},
		{"https://gitlab.com/grp/proj/-/tree/main", "gitlab", "grp/proj"},
	}
	for _, c := range cases {
		p, f, err := ParseRepoRef(c.in)
		if err != nil || p != c.provider || f != c.full {
			t.Errorf("ParseRepoRef(%q) = %q %q %v, want %q %q", c.in, p, f, err, c.provider, c.full)
		}
	}
	for _, bad := range []string{"https://bitbucket.org/a/b", "justaname", ""} {
		if _, _, err := ParseRepoRef(bad); err == nil {
			t.Errorf("ParseRepoRef(%q) should fail", bad)
		}
	}
}

// templateSpec mirrors SPEC_TEMPLATE in web/lib/deploy-spec.ts.
const templateSpec = `# croncompose.yml — full reference at /docs
version: 1
repo: https://github.com/acme/shop
branch: main
server: my-server            # server name or id in CronCompose

install: npm ci && npm run build
port: 3000
process_manager: pm2         # none | pm2 | systemd | docker

env:
  NODE_ENV: production

health:
  path: /healthz
auto_rollback: true
`

func TestParseSpecFileTemplate(t *testing.T) {
	res := ParseSpecFile([]byte(templateSpec))
	if !res.Valid || len(res.Issues) != 0 {
		t.Fatalf("template should parse cleanly: %+v", res.Issues)
	}
	if res.Spec.Install != "npm ci && npm run build" || res.Spec.Server != "my-server" {
		t.Errorf("template = %+v", res.Spec)
	}
}
