package connectors

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

const systemdConfigRoot = "/etc/systemd/system"

func systemdOwns(path string) bool {
	clean := filepath.Clean(path)
	root := filepath.Clean(systemdConfigRoot)
	return clean == root || strings.HasPrefix(clean, root+string(os.PathSeparator))
}

// systemdUnitFromPath maps a unit file or drop-in back to the unit systemctl talks
// about. A path that is not a unit (the directory itself, a loose .conf) returns "".
func systemdUnitFromPath(path string) string {
	clean := filepath.Clean(path)
	dirName := filepath.Base(filepath.Dir(clean))
	if strings.HasSuffix(dirName, ".d") {
		unit := strings.TrimSuffix(dirName, ".d")
		if isSystemdUnitName(unit) {
			return unit
		}
		return ""
	}
	base := filepath.Base(clean)
	if isSystemdUnitName(base) {
		return base
	}
	return ""
}

func isSystemdUnitName(name string) bool {
	for _, ext := range []string{
		".service", ".socket", ".timer", ".target", ".path",
		".mount", ".automount", ".swap", ".slice", ".scope",
	} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

// systemdConfigFiles lists local unit files and their drop-ins under root. Vendor
// units in /lib and /usr/lib are intentionally absent: those are not ours to edit.
func systemdConfigFiles(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		p := filepath.Join(root, name)
		if e.IsDir() {
			unit := strings.TrimSuffix(name, ".d")
			if unit == name || !isSystemdUnitName(unit) {
				continue
			}
			subs, err := os.ReadDir(p)
			if err != nil {
				continue
			}
			for _, s := range subs {
				if s.IsDir() || !strings.HasSuffix(s.Name(), ".conf") {
					continue
				}
				out = append(out, filepath.Join(p, s.Name()))
				if len(out) >= 50 {
					return out
				}
			}
			continue
		}
		if isSystemdUnitName(name) {
			out = append(out, p)
			if len(out) >= 50 {
				return out
			}
		}
	}
	return out
}

func (p *systemdProvider) ReadConfig(ctx context.Context, inst Instance, path string) Result {
	if !systemdOwns(path) {
		return fail(StatusUnauthorized, "path is outside /etc/systemd/system: "+path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fail(StatusFailed, "read failed: "+err.Error())
	}
	return configBytesResult(path, b)
}

func (p *systemdProvider) Validate(ctx context.Context, inst Instance, path string, content []byte) Result {
	if !systemdOwns(path) {
		return fail(StatusUnauthorized, "path is outside /etc/systemd/system: "+path)
	}
	if len(content) == 0 {
		return fail(StatusInvalid, "unit file is empty")
	}
	if strings.IndexByte(string(content), 0) >= 0 {
		return fail(StatusInvalid, "unit file contains a NUL byte")
	}
	unit := systemdUnitFromPath(path)
	if unit == "" || strings.HasSuffix(filepath.Base(path), ".conf") {
		return ok("drop-in looks like text; systemd-analyze verify runs against the merged unit after the write")
	}
	if !has("systemd-analyze") {
		return ok("structure looks sane; systemd-analyze is not installed, so live verify is skipped")
	}
	tmp, err := os.CreateTemp("", "croncompose-*.service")
	if err != nil {
		return fail(StatusFailed, "could not stage candidate: "+err.Error())
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fail(StatusFailed, "could not stage candidate: "+err.Error())
	}
	tmp.Close()
	out, err := run(ctx, "systemd-analyze", "verify", tmp.Name())
	if err != nil {
		return fail(StatusInvalid, trimOutput(out))
	}
	return ok("systemd-analyze verify accepted the candidate")
}

func (p *systemdProvider) ValidateLive(ctx context.Context, inst Instance, path string) Result {
	if !systemdOwns(path) {
		return fail(StatusUnauthorized, "path is outside /etc/systemd/system: "+path)
	}
	if !has("systemd-analyze") {
		return ok("systemd-analyze is not installed; skipped live verify")
	}
	target := path
	if strings.HasSuffix(filepath.Base(path), ".conf") {
		if unit := systemdUnitFromPath(path); unit != "" {
			target = unit
		}
	}
	out, err := run(ctx, "systemd-analyze", "verify", target)
	if err != nil && needsPriv() {
		var ran bool
		out, ran, err = privRun(ctx, "systemd-analyze", "verify", target)
		if !ran {
			return unauthorized("systemd-analyze")
		}
	}
	if err != nil {
		return fail(StatusInvalid, trimOutput(out))
	}
	return ok("systemd-analyze verify passed")
}

// Activate re-reads unit files, then tries to pick up the change on the affected
// unit without starting a unit that was stopped. try-reload-or-restart is a no-op
// for inactive units and a reload (or restart, if the unit has no reload) for
// active ones.
func (p *systemdProvider) Activate(ctx context.Context, inst Instance, path string) Result {
	if !systemdAvailable() {
		return noSystemd("reload units")
	}
	out, ran, err := privRun(ctx, "systemctl", "daemon-reload")
	if !ran {
		return unauthorized("systemctl")
	}
	reloadStep := step("systemctl daemon-reload", err == nil, out)
	if err != nil {
		return fail(StatusFailed, "daemon-reload failed: "+trimOutput(out), reloadStep)
	}
	unit := systemdUnitFromPath(path)
	if unit == "" {
		return ok("unit files reloaded", reloadStep)
	}
	out2, ran2, err2 := privRun(ctx, "systemctl", "try-reload-or-restart", unit)
	if !ran2 {
		return unauthorized("systemctl")
	}
	applyStep := step("systemctl try-reload-or-restart "+unit, err2 == nil, out2)
	if err2 != nil {
		return fail(StatusFailed, "try-reload-or-restart "+unit+" failed: "+trimOutput(out2), reloadStep, applyStep)
	}
	return ok("daemon reloaded; "+unit+" picked up the change", reloadStep, applyStep)
}
