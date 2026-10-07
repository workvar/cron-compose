package deploys

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// SpecVersion is the newest croncompose.yml schema this control plane understands.
// Files without a version key are treated as version 1: that is the shape
// CronCompose has always written into imported repos.
const SpecVersion = 1

// SpecFileNames are the basename paths the importer looks for, in order.
// They may sit at the repo root or in a subfolder (monorepos).
var SpecFileNames = []string{"croncompose.yml", "croncompose.yaml", ".croncompose.yml", ".croncompose.yaml"}

// IsSpecFileName reports whether path's basename is a recognized croncompose.yml.
func IsSpecFileName(path string) bool {
	base := path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		base = path[i+1:]
	}
	for _, n := range SpecFileNames {
		if base == n {
			return true
		}
	}
	return false
}

// SpecIssue is one problem found while reading a croncompose.yml. Errors block a
// deploy; warnings are shown next to the form and do not.
type SpecIssue struct {
	Level   string `json:"level"` // "error" or "warning"
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// SpecResult is POST /deploys/spec/validate and the spec part of GET /git/inspect.
type SpecResult struct {
	Path   string      `json:"path,omitempty"`
	Spec   Spec        `json:"spec"`
	Issues []SpecIssue `json:"issues"`
	Valid  bool        `json:"valid"`
}

// Supported values, kept here so the parser and the public docs agree.
// Framework ids (nextjs, nestjs, …) are first-class; clone paths resolve via RuntimeLanguage.
var (
	SpecLanguages = []string{
		"node", "nextjs", "nestjs", "react", "vue", "nuxt", "remix", "sveltekit", "astro", "express",
		"typescript", "javascript", "bun", "deno",
		"python", "fastapi", "django", "flask",
		"go", "rust",
		"dotnet", "csharp",
		"ruby", "rails", "php", "laravel", "elixir",
		"java", "spring", "kotlin",
		"docker", "unknown",
	}
	SpecProcessManagers = []string{"none", "pm2", "systemd", "docker"}
)

var (
	envKeyPattern   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	secretKeyHint   = regexp.MustCompile(`(?i)(SECRET|TOKEN|PASSWORD|PASSWD|PRIVATE|API_?KEY|ACCESS_?KEY|CREDENTIAL)`)
	repoPathPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)+$`)
	unknownKeyMsg   = regexp.MustCompile(`line (\d+): field (\S+) not found in type`)
)

// specFile mirrors Spec but lets app env be written either as a map or as a list of
// {key, value} pairs. The map form is what people type; the list form is what
// CronCompose itself writes back.
type specFile struct {
	Version        int               `yaml:"version"`
	Name           string            `yaml:"name"`
	Provider       string            `yaml:"provider"`
	Repo           string            `yaml:"repo"`
	Branch         string            `yaml:"branch"`
	Server         string            `yaml:"server"`
	Language       string            `yaml:"language"`
	Install        string            `yaml:"install"`
	Run            string            `yaml:"run"`
	Root           string            `yaml:"root"`
	Port           int               `yaml:"port"`
	ProcessManager string            `yaml:"process_manager"`
	ClonePath      string            `yaml:"clone_path"`
	Env            map[string]string `yaml:"env"`
	Health         *SpecHealth       `yaml:"health"`
	DeployTimeout  int               `yaml:"deploy_timeout"`
	AutoRollback   bool              `yaml:"auto_rollback"`
	RedeployOn     []string          `yaml:"redeploy_on"`
	Apps           []specFileApp     `yaml:"apps"`
}

type specFileApp struct {
	Name           string    `yaml:"name"`
	Root           string    `yaml:"root"`
	Language       string    `yaml:"language"`
	Install        string    `yaml:"install"`
	Run            string    `yaml:"run"`
	Port           int       `yaml:"port"`
	ProcessManager string    `yaml:"process_manager"`
	Env            yaml.Node `yaml:"env"`
}

// ParseSpecFile reads a croncompose.yml, normalizes it, and reports every problem it
// can find in one pass so the UI can show them all at once.
func ParseSpecFile(raw []byte) SpecResult {
	res := SpecResult{Issues: []SpecIssue{}}
	add := func(level, field, format string, args ...any) {
		res.Issues = append(res.Issues, SpecIssue{Level: level, Field: field, Message: fmt.Sprintf(format, args...)})
	}

	if strings.TrimSpace(string(raw)) == "" {
		add("error", "", "file is empty")
		return res
	}

	var f specFile
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		var te *yaml.TypeError
		if !errors.As(err, &te) {
			add("error", "", "not valid YAML: %s", strings.TrimPrefix(err.Error(), "yaml: "))
			return res
		}
		// yaml.v3 keeps decoding past type errors, so f holds everything else.
		for _, msg := range te.Errors {
			if m := unknownKeyMsg.FindStringSubmatch(msg); m != nil {
				add("warning", m[2], "unknown key %q on line %s was ignored; check the spelling", m[2], m[1])
			} else {
				add("error", "", "%s", trimYAMLLine(msg))
			}
		}
	}

	s := Spec{
		Version: f.Version, Name: strings.TrimSpace(f.Name), Provider: strings.ToLower(strings.TrimSpace(f.Provider)),
		Repo: strings.TrimSpace(f.Repo), Branch: strings.TrimSpace(f.Branch), Server: strings.TrimSpace(f.Server),
		Language: strings.ToLower(strings.TrimSpace(f.Language)), Install: strings.TrimSpace(f.Install),
		Run: strings.TrimSpace(f.Run),
		Root: f.Root, Port: f.Port, ProcessManager: strings.ToLower(strings.TrimSpace(f.ProcessManager)),
		ClonePath: strings.TrimSpace(f.ClonePath), Env: f.Env, Health: f.Health,
		DeployTimeout: f.DeployTimeout, AutoRollback: f.AutoRollback,
		RedeployOn: NormalizeRedeployOn(f.RedeployOn),
	}

	switch {
	case s.Version == 0:
		s.Version = SpecVersion
	case s.Version > SpecVersion:
		add("error", "version", "version %d is newer than this CronCompose understands (max %d); update CronCompose", s.Version, SpecVersion)
	case s.Version < 0:
		add("error", "version", "version must be %d", SpecVersion)
	}

	if s.Repo != "" {
		provider, full, err := ParseRepoRef(s.Repo)
		if err != nil {
			add("error", "repo", "%s", err.Error())
		} else {
			if s.Provider != "" && provider != "" && s.Provider != provider {
				add("error", "provider", "provider %q does not match repo host (%s)", s.Provider, provider)
			}
			if s.Provider == "" {
				s.Provider = provider
			}
			s.Repo = full
		}
	}
	if s.Provider == "" {
		s.Provider = "github"
	}
	if s.Provider != "github" && s.Provider != "gitlab" {
		add("error", "provider", "provider must be github or gitlab, got %q", s.Provider)
	}

	if s.Root != "" {
		root, err := cleanSpecRoot(s.Root)
		if err != nil {
			add("error", "root", "%s", err.Error())
		}
		s.Root = root
	}
	checkLanguage(add, "language", s.Language)
	checkProcessManager(add, "process_manager", s.ProcessManager)
	checkPort(add, "port", s.Port)
	if s.ClonePath != "" && !strings.HasPrefix(s.ClonePath, "/") {
		add("error", "clone_path", "clone_path must be an absolute path (start with /)")
	}
	if s.DeployTimeout < 0 {
		add("error", "deploy_timeout", "deploy_timeout must be 0 or more seconds")
	}
	if s.Health != nil {
		s.Health.Path = strings.TrimSpace(s.Health.Path)
		if s.Health.Path == "" {
			add("error", "health.path", "health.path is required when health is set")
		} else if !strings.HasPrefix(s.Health.Path, "/") {
			add("error", "health.path", "health.path must start with /")
		}
		checkPort(add, "health.port", s.Health.Port)
		if s.Health.Timeout < 0 {
			add("error", "health.timeout", "health.timeout must be 0 or more seconds")
		}
	}
	checkEnv(add, "env", s.Env)

	names := map[string]bool{}
	roots := map[string]bool{}
	for i, a := range f.Apps {
		field := fmt.Sprintf("apps[%d]", i)
		app := SpecApp{
			Name: strings.TrimSpace(a.Name), Language: strings.ToLower(strings.TrimSpace(a.Language)),
			Install: strings.TrimSpace(a.Install), Run: strings.TrimSpace(a.Run), Port: a.Port,
			ProcessManager: strings.ToLower(strings.TrimSpace(a.ProcessManager)),
		}
		root, err := cleanSpecRoot(a.Root)
		if err != nil {
			add("error", field+".root", "%s", err.Error())
		}
		app.Root = root
		if app.Name == "" {
			app.Name = appNameFromRoot(root, s.Name, s.Repo)
		}
		if names[app.Name] {
			add("error", field+".name", "two apps are named %q; names must be unique", app.Name)
		}
		names[app.Name] = true
		if roots[root] {
			add("error", field+".root", "two apps use root %q; each app needs its own folder", root)
		}
		roots[root] = true
		checkLanguage(add, field+".language", app.Language)
		checkProcessManager(add, field+".process_manager", app.ProcessManager)
		checkPort(add, field+".port", app.Port)
		env, err := decodeAppEnv(a.Env)
		if err != nil {
			add("error", field+".env", "%s", err.Error())
		}
		envMap := map[string]string{}
		for _, v := range env {
			envMap[v.Key] = v.Value
		}
		checkEnv(add, field+".env", envMap)
		app.Env = env
		s.Apps = append(s.Apps, app)
	}

	if s.Port != 0 && len(s.Apps) > 1 {
		add("warning", "port", "top-level port is ignored with more than one app; set port on each app")
	}

	res.Spec = s
	res.Valid = true
	for _, is := range res.Issues {
		if is.Level == "error" {
			res.Valid = false
			break
		}
	}
	return res
}

// ParseRepoRef accepts "owner/name", "owner/group/name", or an https / ssh clone
// URL, and returns the provider it implies ("" for a bare path) and the full name.
func ParseRepoRef(ref string) (provider, fullName string, err error) {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "git@") {
		// git@github.com:owner/name.git
		rest := strings.TrimPrefix(ref, "git@")
		host, p, ok := strings.Cut(rest, ":")
		if !ok {
			return "", "", fmt.Errorf("repo %q is not a recognised git URL", ref)
		}
		ref = "https://" + host + "/" + p
	}
	if strings.Contains(ref, "://") {
		u, perr := url.Parse(ref)
		if perr != nil || u.Host == "" {
			return "", "", fmt.Errorf("repo %q is not a valid URL", ref)
		}
		switch strings.ToLower(strings.TrimPrefix(u.Host, "www.")) {
		case "github.com":
			provider = "github"
		case "gitlab.com":
			provider = "gitlab"
		default:
			return "", "", fmt.Errorf("repo host %q is not supported; use github.com or gitlab.com, or owner/name with provider set", u.Host)
		}
		ref = u.Path
	}
	full := strings.Trim(strings.TrimSuffix(strings.Trim(ref, "/"), ".git"), "/")
	if i := strings.Index(full, "/-/"); i >= 0 { // GitLab web URLs: /group/proj/-/tree/main
		full = full[:i]
	}
	if provider == "github" {
		parts := strings.Split(full, "/")
		if len(parts) > 2 { // /owner/name/tree/main
			full = parts[0] + "/" + parts[1]
		}
	}
	if !repoPathPattern.MatchString(full) {
		return "", "", fmt.Errorf("repo must look like owner/name, got %q", ref)
	}
	return provider, full, nil
}

func cleanSpecRoot(root string) (string, error) {
	r := strings.TrimSpace(strings.ReplaceAll(root, `\`, "/"))
	if strings.HasPrefix(r, "/") {
		return ".", fmt.Errorf("root %q must be relative to the repo (no leading /)", root)
	}
	if r == "" {
		return ".", nil
	}
	clean := path.Clean(r)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return ".", fmt.Errorf("root %q points outside the repo", root)
	}
	return clean, nil
}

func appNameFromRoot(root, specName, repo string) string {
	if root != "." && root != "" {
		return path.Base(root)
	}
	if specName != "" {
		return specName
	}
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		return repo[i+1:]
	}
	return "app"
}

func decodeAppEnv(n yaml.Node) ([]EnvVar, error) {
	switch n.Kind {
	case 0:
		return nil, nil
	case yaml.MappingNode:
		var m map[string]string
		if err := n.Decode(&m); err != nil {
			return nil, fmt.Errorf("env values must be strings: %s", trimYAMLLine(err.Error()))
		}
		out := make([]EnvVar, 0, len(m))
		// Keep file order: yaml.Node content alternates key, value.
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i].Value
			out = append(out, EnvVar{Key: k, Value: m[k]})
		}
		return out, nil
	case yaml.SequenceNode:
		var list []struct {
			Key   string `yaml:"key"`
			Value string `yaml:"value"`
		}
		if err := n.Decode(&list); err != nil {
			return nil, fmt.Errorf("env list items need key and value: %s", trimYAMLLine(err.Error()))
		}
		out := make([]EnvVar, 0, len(list))
		for _, v := range list {
			out = append(out, EnvVar{Key: v.Key, Value: v.Value})
		}
		return out, nil
	default:
		return nil, errors.New("env must be a map (KEY: value) or a list of {key, value}")
	}
}

type addIssue func(level, field, format string, args ...any)

func checkLanguage(add addIssue, field, lang string) {
	if lang == "" || contains(SpecLanguages, lang) {
		return
	}
	// Unknown custom ids still work if they map onto a known runtime clone path.
	if rt := RuntimeLanguage(lang); rt != lang && contains(SpecLanguages, rt) {
		return
	}
	add("warning", field, "language %q is not a known framework/runtime; clone path falls back to /opt/apps", lang)
}

func checkProcessManager(add addIssue, field, pm string) {
	if pm != "" && !contains(SpecProcessManagers, pm) {
		add("error", field, "process_manager must be one of %s, got %q", strings.Join(SpecProcessManagers, ", "), pm)
	}
}

func checkPort(add addIssue, field string, port int) {
	if port < 0 || port > 65535 {
		add("error", field, "port must be between 1 and 65535, got %d", port)
	}
}

func checkEnv(add addIssue, field string, env map[string]string) {
	for k, v := range env {
		if !envKeyPattern.MatchString(k) {
			add("error", field, "env key %q must be letters, digits and _ and not start with a digit", k)
			continue
		}
		if v != "" && secretKeyHint.MatchString(k) {
			add("warning", field, "%s looks like a secret; leave it out of the file and add it as a sensitive variable in CronCompose", k)
		}
	}
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func trimYAMLLine(msg string) string {
	return strings.TrimSpace(strings.TrimPrefix(msg, "yaml: "))
}
