package deploys

import "testing"

func TestDetectFromFiles(t *testing.T) {
	cases := []struct {
		name       string
		files      map[string]string
		language   string
		install    string
		pm2        bool
		portOK     bool
		workspaces []string
	}{
		{
			name: "next.js",
			files: map[string]string{
				"package.json": `{"name":"web","dependencies":{"next":"16.0.0"}}`,
			},
			language: "node",
			install:  "npm install && npm run build",
			portOK:   true,
		},
		{
			name: "pnpm workspace monorepo",
			files: map[string]string{
				"package.json":          `{"name":"root","private":true,"workspaces":["apps/*","packages/*"]}`,
				"pnpm-workspace.yaml":   "packages:\n  - apps/*\n",
				"apps/web/package.json": `{"name":"web"}`,
				"apps/api/package.json": `{"name":"api"}`,
			},
			language:   "node",
			install:    "pnpm install && pnpm run build",
			workspaces: []string{"apps/api", "apps/web"},
		},
		{
			name: "python",
			files: map[string]string{
				"pyproject.toml":   "[project]\nname = 'svc'\n",
				"requirements.txt": "flask==3.0.0\n",
			},
			language: "python",
			install:  "python3 -m venv .venv && . .venv/bin/activate && pip install -r requirements.txt",
		},
		{
			name:     "go",
			files:    map[string]string{"go.mod": "module example.com/app\n"},
			language: "go",
			install:  "go build -o app .",
		},
		{
			name: "pm2 ecosystem",
			files: map[string]string{
				"package.json":        `{"name":"api"}`,
				"ecosystem.config.js": "module.exports = { apps: [{ name: 'api' }] }",
			},
			language: "node",
			install:  "npm install",
			pm2:      true,
			portOK:   true,
		},
		{
			name:     "docker compose",
			files:    map[string]string{"docker-compose.yml": "services:\n  web: {}\n"},
			language: "docker",
			install:  "docker compose up -d",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Detect(c.files)
			if got.Language != c.language {
				t.Errorf("language = %q, want %q", got.Language, c.language)
			}
			if got.InstallScript != c.install {
				t.Errorf("install = %q, want %q", got.InstallScript, c.install)
			}
			if got.HasPM2Ecosystem != c.pm2 {
				t.Errorf("pm2 = %v, want %v", got.HasPM2Ecosystem, c.pm2)
			}
			if got.SupportsPort != c.portOK && c.portOK {
				t.Errorf("supports port = %v, want %v", got.SupportsPort, c.portOK)
			}
			if len(c.workspaces) > 0 {
				if len(got.Workspaces) != len(c.workspaces) {
					t.Fatalf("workspaces = %v, want %v", got.Workspaces, c.workspaces)
				}
				for i := range c.workspaces {
					if got.Workspaces[i] != c.workspaces[i] {
						t.Errorf("workspaces[%d] = %q, want %q", i, got.Workspaces[i], c.workspaces[i])
					}
				}
			}
		})
	}
}

func TestClonePath(t *testing.T) {
	paths := DefaultLanguagePaths()
	got := ClonePath(paths, "node", "acme/web-app")
	want := "/opt/apps/node/web-app"
	if got != want {
		t.Errorf("ClonePath = %q, want %q", got, want)
	}
	custom := map[string]string{"node": "/home/pi/src"}
	if got := ClonePath(custom, "node", "foo/bar"); got != "/home/pi/src/bar" {
		t.Errorf("custom path = %q", got)
	}
}

func TestSanitizeRepoName(t *testing.T) {
	if got := repoDirName("Acme/my app!"); got != "my-app" {
		t.Errorf("repoDirName = %q", got)
	}
}
