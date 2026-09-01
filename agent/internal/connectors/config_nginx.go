package connectors

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// mainConfigOf returns the path of the file nginx reads first on this host. Detect
// records the prefix it found, so a command acts on the same tree discovery reported
// rather than re-deriving it and possibly disagreeing. The fallback re-probes for the
// case of an instance that reached here without going through Detect.
func mainConfigOf(inst Instance) string {
	if prefix := inst.Detail["config_prefix"]; prefix != "" {
		return nginxMainConfig(prefix)
	}
	return nginxMainConfig(nginxPrefix())
}

// ReadConfig returns one nginx config file. The path is confined to the config paths
// discovery reported, so a compromised control plane cannot use this connector as a
// general-purpose file reader.
func (p *nginxProvider) ReadConfig(ctx context.Context, inst Instance, path string) Result {
	if !p.owns(inst, path) {
		return fail(StatusUnauthorized, "path is outside this connector's config paths: "+path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fail(StatusFailed, "read failed: "+err.Error())
	}
	return configBytesResult(path, b)
}

// Validate checks a candidate without touching the live tree.
//
// For the main config that is a real check: nginx will parse the candidate in full
// with `-t -c`. For an include it is not possible in isolation, so we do the cheap
// structural check we can do honestly (balanced braces) and say plainly that the real
// validation happens after the write, where a failure rolls back.
func (p *nginxProvider) Validate(ctx context.Context, inst Instance, path string, content []byte) Result {
	if !p.owns(inst, path) {
		return fail(StatusUnauthorized, "path is outside this connector's config paths: "+path)
	}

	if main := mainConfigOf(inst); main != "" && filepath.Clean(path) == filepath.Clean(main) {
		tmp, err := os.CreateTemp("", "croncompose-nginx-*.conf")
		if err != nil {
			return fail(StatusFailed, "could not stage candidate: "+err.Error())
		}
		defer os.Remove(tmp.Name())
		if _, err := tmp.Write(content); err != nil {
			tmp.Close()
			return fail(StatusFailed, "could not stage candidate: "+err.Error())
		}
		tmp.Close()

		out, err := run(ctx, "nginx", "-t", "-c", tmp.Name())
		if err != nil {
			return fail(StatusInvalid, trimOutput(out))
		}
		return ok("nginx -t accepted the candidate")
	}

	if err := balancedBraces(content); err != nil {
		return fail(StatusInvalid, err.Error())
	}
	return ok("structure looks sane; nginx -t runs against the full tree after the write")
}

// ValidateLive runs nginx's own checker over the configuration currently on disk.
func (p *nginxProvider) ValidateLive(ctx context.Context, inst Instance, path string) Result {
	out, err := run(ctx, "nginx", "-t")
	if err != nil {
		return fail(StatusInvalid, trimOutput(out))
	}
	return ok("nginx -t passed")
}

// Activate reloads nginx. Reload keeps existing connections alive, which is the whole
// reason to prefer it over a restart for a config change.
//
// systemctl is tried first because on a systemd box it is what the operator would run,
// and it keeps the unit's view of the service accurate. It is not the only way in:
// containers and non-systemd distributions have no systemctl, or have one that cannot
// reach a bus, so a FAILURE there falls through to nginx's own signal handling rather
// than being reported as the end of the story.
func (p *nginxProvider) Activate(ctx context.Context, inst Instance, path string) Result {
	var systemctlOut string
	// Skipping systemctl outright where there is no systemd keeps the failure detail
	// clean: on macOS the first attempt would otherwise contribute nothing but an
	// "executable file not found" string to the message for a reload that then works.
	if systemdAvailable() {
		out, ran, err := privRun(ctx, "systemctl", "reload", "nginx")
		if ran && err == nil {
			return ok("nginx reloaded via systemctl")
		}
		systemctlOut = out
	}

	out, ran, err := privRun(ctx, "nginx", "-s", "reload")
	if !ran {
		return unauthorized("systemctl or nginx")
	}
	if err == nil {
		return ok("nginx reloaded")
	}
	detail := trimOutput(out)
	if systemctlOut != "" {
		detail = "systemctl: " + trimOutput(systemctlOut) + "; nginx -s reload: " + detail
	}
	return fail(StatusFailed, "reload failed: "+detail)
}

// Lifecycle drives the nginx service itself, as opposed to its config files.
func (p *nginxProvider) Lifecycle(ctx context.Context, inst Instance, ref, action string) Result {
	if !ValidAction(action) {
		return fail(StatusUnsupported, "unknown action: "+action)
	}
	if !systemdAvailable() {
		return noSystemd(action + " nginx")
	}
	out, ran, err := privRun(ctx, "systemctl", action, "nginx")
	if !ran {
		return unauthorized("systemctl")
	}
	s := step("systemctl "+action+" nginx", err == nil, out)
	if err != nil {
		return fail(StatusFailed, "systemctl "+action+" nginx failed: "+trimOutput(out), s)
	}
	return ok("nginx "+action+"ed", s, step("status", true, serviceStatus(ctx, "nginx")))
}

// owns confines every file operation to the directories discovery reported. Without
// this the connector would be an arbitrary-file read/write primitive wearing an nginx
// label.
func (p *nginxProvider) owns(inst Instance, path string) bool {
	clean := filepath.Clean(path)
	for _, base := range inst.ConfigPaths {
		base = filepath.Clean(base)
		if clean == base || strings.HasPrefix(clean, base+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
