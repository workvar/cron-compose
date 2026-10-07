package connectors

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type systemdProvider struct{}

func (systemdProvider) Kind() string { return "systemd" }

func (p *systemdProvider) Detect(ctx context.Context) []Instance {
	// systemctl on PATH is not the same as systemd running: a container ships the
	// package without PID 1, and reporting an instance there would offer the operator
	// lifecycle buttons that can only fail. macOS has neither, and the same check
	// excludes it.
	if !systemdAvailable() {
		return nil
	}
	ver := ""
	if out, err := run(ctx, "systemctl", "--version"); err == nil {
		ver = secondField(out) // "systemd 255 (...)" -> "255"
	}
	count := 0
	if out, err := p.listUnits(ctx); err == nil {
		count = countLines(out)
	}
	// Lifecycle and unit-file edits need root, or a passwordless sudoers grant for
	// systemctl. Report what this agent can actually do, not what it wishes it could.
	manageable := os.Geteuid() == 0 || canSudo(ctx, "systemctl")
	paths := existingPaths(systemdConfigRoot)
	return []Instance{{
		Kind:        "systemd",
		Version:     ver,
		Status:      "running",
		ObjectCount: count,
		ConfigPaths: paths,
		Manageable:  manageable,
		Caps: Capabilities{
			ManagesObjects: true,
			ManagesConfig:  len(paths) > 0,
			CanValidate:    has("systemd-analyze"),
			CanReload:      manageable,
			CanLifecycle:   manageable,
			CanEdit:        manageable && len(paths) > 0,
		},
		Detail: map[string]string{},
	}}
}

func (p *systemdProvider) Resources(ctx context.Context, inst Instance) []Resource {
	details := p.showAll(ctx)
	res := []Resource{}
	if out, err := p.listUnits(ctx); err == nil {
		for _, line := range strings.Split(out, "\n") {
			// Columns (--plain --no-legend): UNIT LOAD ACTIVE SUB DESCRIPTION...
			f := strings.Fields(line)
			if len(f) < 4 {
				continue
			}
			unit := f[0]
			state := "stopped"
			if f[2] == "active" {
				state = "running"
			}
			attrs := map[string]string{
				"load":   f[1],
				"active": f[2],
				"sub":    f[3],
			}
			if d, ok := details[unit]; ok {
				for k, v := range d {
					attrs[k] = v
				}
			}
			res = append(res, Resource{
				Type:       "object",
				Ref:        unit,
				Name:       strings.TrimSuffix(unit, ".service"),
				State:      state,
				Attributes: attrs,
			})
			if len(res) >= 250 {
				break
			}
		}
	}
	root := systemdConfigRoot
	if len(inst.ConfigPaths) > 0 && inst.ConfigPaths[0] != "" {
		root = inst.ConfigPaths[0]
	}
	for _, f := range systemdConfigFiles(root) {
		sum, size := fileChecksum(f)
		res = append(res, Resource{
			Type:       "config_file",
			Ref:        f,
			Name:       filepath.Base(f),
			Checksum:   sum,
			SizeBytes:  size,
			Attributes: map[string]string{},
		})
	}
	return res
}

// showAll batches ExecStart / WorkingDirectory / FragmentPath for all services.
// Environment values are not included here — only env_count via Inspect.
func (p *systemdProvider) showAll(ctx context.Context) map[string]map[string]string {
	out, err := run(ctx, "systemctl", "show", "--type=service", "--all", "--no-pager",
		"-p", "Id", "-p", "FragmentPath", "-p", "ExecStart", "-p", "WorkingDirectory",
		"-p", "Environment")
	if err != nil {
		return nil
	}
	return parseSystemdShowBlocks(out)
}

func parseSystemdShowBlocks(out string) map[string]map[string]string {
	blocks := map[string]map[string]string{}
	cur := map[string]string{}
	flush := func() {
		id := cur["Id"]
		if id == "" {
			cur = map[string]string{}
			return
		}
		attrs := map[string]string{}
		if v := cur["FragmentPath"]; v != "" {
			attrs["unit_path"] = v
		}
		if v := cur["WorkingDirectory"]; v != "" && v != "/" {
			attrs["cwd"] = v
		}
		if v := cur["ExecStart"]; v != "" {
			attrs["command"] = stripSystemdExecStart(v)
			attrs["exec"] = attrs["command"]
		}
		if v := cur["Environment"]; v != "" {
			env := parseSystemdEnvironment(v)
			attrs["env_count"] = strconv.Itoa(len(env))
			attrs["env_keys"] = envKeysCSV(env)
		}
		blocks[id] = attrs
		cur = map[string]string{}
	}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			flush()
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		cur[k] = v
	}
	flush()
	return blocks
}

// stripSystemdExecStart turns "{ path=/usr/bin/foo ; argv[]=/usr/bin/foo -c bar ; ... }"
// into a readable command line.
func stripSystemdExecStart(raw string) string {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "argv[]="); i >= 0 {
		rest := raw[i+len("argv[]="):]
		if j := strings.Index(rest, " ;"); j >= 0 {
			rest = rest[:j]
		}
		return strings.TrimSpace(rest)
	}
	if i := strings.Index(raw, "path="); i >= 0 {
		rest := raw[i+len("path="):]
		if j := strings.IndexAny(rest, " ;"); j >= 0 {
			rest = rest[:j]
		}
		return strings.TrimSpace(rest)
	}
	return raw
}

func parseSystemdEnvironment(raw string) map[string]string {
	// Environment=FOO=1 BAR=2 (space-separated KEY=VAL)
	return parseEnvList(strings.Fields(raw))
}

func (p *systemdProvider) showOne(ctx context.Context, unit string) map[string]string {
	out, err := run(ctx, "systemctl", "show", unit, "--no-pager",
		"-p", "Id", "-p", "FragmentPath", "-p", "ExecStart", "-p", "WorkingDirectory",
		"-p", "Environment", "-p", "EnvironmentFiles", "-p", "ActiveState", "-p", "Description")
	if err != nil {
		return nil
	}
	cur := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(line, "=")
		if ok {
			cur[k] = v
		}
	}
	return cur
}

// Inspect returns ExecStart, cwd, and Environment for one unit.
func (p *systemdProvider) Inspect(ctx context.Context, inst Instance, ref string) Result {
	if !systemdAvailable() {
		return noSystemd("inspect " + ref)
	}
	cur := p.showOne(ctx, ref)
	if cur == nil || cur["Id"] == "" {
		return fail(StatusFailed, "systemd unit not found: "+ref)
	}
	env := parseSystemdEnvironment(cur["Environment"])
	// Best-effort EnvironmentFiles (space-separated paths, sometimes prefixed with -)
	if files := strings.Fields(cur["EnvironmentFiles"]); len(files) > 0 {
		for _, f := range files {
			f = strings.TrimPrefix(f, "-")
			f = strings.Trim(f, "()")
			if b, err := os.ReadFile(f); err == nil {
				for _, v := range ParseDotEnvLike(string(b)) {
					if _, exists := env[v.Key]; !exists {
						env[v.Key] = v.Value
					}
				}
			}
		}
	}
	name := strings.TrimSuffix(cur["Id"], ".service")
	cwd := cur["WorkingDirectory"]
	if cwd == "/" {
		cwd = ""
	}
	return inspectJSON(ProcessDetail{
		Name:    name,
		Command: stripSystemdExecStart(cur["ExecStart"]),
		Cwd:     cwd,
		State:   cur["ActiveState"],
		Env:     env,
		Extra: map[string]string{
			"unit_path":   cur["FragmentPath"],
			"description": cur["Description"],
		},
	})
}

func (p *systemdProvider) listUnits(ctx context.Context) (string, error) {
	return run(ctx, "systemctl", "list-units", "--type=service", "--all",
		"--no-legend", "--no-pager", "--plain")
}

func (p *systemdProvider) Ports(ctx context.Context, inst Instance) Result {
	socks := listeningSockets(ctx)
	owners := systemdOwnerMap(ctx)
	return portsResult(attachOwners(socks, owners, os.Getpid()))
}

// envPair is a tiny key/value for EnvironmentFile parsing without importing deploys.
type envPair struct {
	Key, Value string
}

// ParseDotEnvLike parses KEY=VAL lines from an EnvironmentFile.
func ParseDotEnvLike(raw string) []envPair {
	var out []envPair
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 {
			if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
				v = v[1 : len(v)-1]
			}
		}
		out = append(out, envPair{Key: k, Value: v})
	}
	return out
}
