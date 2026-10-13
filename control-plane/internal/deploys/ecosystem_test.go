package deploys

import (
	"testing"
)

func TestParsePM2EcosystemJSON(t *testing.T) {
	apps := ParsePM2Ecosystem(`{
  "apps": [{
    "name": "peepal-proxy",
    "script": "./proxy",
    "env": { "PORT": "8080" }
  }]
}`)
	if len(apps) != 1 {
		t.Fatalf("len = %d, want 1", len(apps))
	}
	if apps[0].Name != "peepal-proxy" || apps[0].Script != "./proxy" || apps[0].Port != 8080 {
		t.Fatalf("got %+v", apps[0])
	}
}

func TestParsePM2EcosystemJS(t *testing.T) {
	body := `
// peepal reverse proxy
module.exports = {
  apps: [{
    name: 'peepal-proxy',
    script: './go',
    args: 'run .',
    cwd: 'proxy',
    env: {
      PORT: 9090,
    },
  }],
};
`
	apps := ParsePM2Ecosystem(body)
	if len(apps) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(apps), apps)
	}
	a := apps[0]
	if a.Name != "peepal-proxy" {
		t.Errorf("name = %q", a.Name)
	}
	if a.Script != "./go" {
		t.Errorf("script = %q", a.Script)
	}
	if a.Args != "run ." {
		t.Errorf("args = %q", a.Args)
	}
	if a.Cwd != "proxy" {
		t.Errorf("cwd = %q", a.Cwd)
	}
	if a.Port != 9090 {
		t.Errorf("port = %d", a.Port)
	}
	if got := RunFromPM2(a); got != "./go run ." {
		t.Errorf("RunFromPM2 = %q", got)
	}
}

func TestParsePM2EcosystemSingleAppObject(t *testing.T) {
	apps := ParsePM2Ecosystem(`module.exports = { apps: { name: "api", script: "npm", args: "start" } }`)
	if len(apps) != 1 || apps[0].Name != "api" || apps[0].Script != "npm" {
		t.Fatalf("got %+v", apps)
	}
}

func TestCollectPM2AppsScoped(t *testing.T) {
	files := map[string]string{
		"ecosystem.config.js": `module.exports = { apps: [
      { name: 'peepal-proxy', script: './proxy', cwd: 'proxy' },
      { name: 'peepal-api', script: './api', cwd: 'api' },
    ]}`,
	}
	proxy := CollectPM2Apps(files, "proxy")
	if len(proxy) != 1 || proxy[0].Name != "peepal-proxy" {
		t.Fatalf("proxy apps = %+v", proxy)
	}
	all := CollectPM2Apps(files, ".")
	if len(all) != 2 {
		t.Fatalf("all apps = %+v", all)
	}
}

func TestCollectPM2AppsNestedFile(t *testing.T) {
	files := map[string]string{
		"proxy/ecosystem.config.js": `module.exports = { apps: [{ name: 'peepal-proxy', script: './proxy' }] }`,
	}
	got := CollectPM2Apps(files, "proxy")
	if len(got) != 1 || got[0].Name != "peepal-proxy" {
		t.Fatalf("got %+v", got)
	}
}

func TestDetectParsesPM2Apps(t *testing.T) {
	d := Detect(map[string]string{
		"go.mod":              "module example.com/proxy\n",
		"ecosystem.config.js": "module.exports = { apps: [{ name: 'peepal-proxy', script: './proxy', env: { PORT: 8080 } }] }",
	})
	if !d.HasPM2Ecosystem {
		t.Fatal("expected HasPM2Ecosystem")
	}
	if len(d.PM2Apps) != 1 || d.PM2Apps[0].Name != "peepal-proxy" {
		t.Fatalf("PM2Apps = %+v", d.PM2Apps)
	}
	if d.PM2Apps[0].Port != 8080 {
		t.Fatalf("port = %d", d.PM2Apps[0].Port)
	}
}

func TestDetectAtRootEcosystemCwd(t *testing.T) {
	files := map[string]string{
		"proxy/go.mod": "module example.com/proxy\n",
		"ecosystem.config.js": `module.exports = { apps: [
      { name: 'peepal-proxy', script: './proxy', cwd: 'proxy' },
      { name: 'peepal-api', script: './api', cwd: 'api' },
    ]}`,
	}
	d := DetectAt(files, "proxy")
	if !d.HasPM2Ecosystem {
		t.Fatal("expected HasPM2Ecosystem")
	}
	if len(d.PM2Apps) != 1 || d.PM2Apps[0].Name != "peepal-proxy" {
		t.Fatalf("PM2Apps = %+v", d.PM2Apps)
	}
}

func TestEnrichProcessNamesPreservesOverride(t *testing.T) {
	apps := EnrichProcessNames("peepal-rp", []SpecApp{
		{Name: "Proxy", ProcessName: "peepal-proxy"},
		{Name: "API"},
	})
	if apps[0].ProcessName != "peepal-proxy" {
		t.Errorf("override lost: %q", apps[0].ProcessName)
	}
	if apps[1].ProcessName != "peepal-rp-api" {
		t.Errorf("qualified = %q", apps[1].ProcessName)
	}
}
