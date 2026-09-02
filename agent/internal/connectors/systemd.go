package connectors

import (
	"context"
	"os"
	"path/filepath"
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
	res := []Resource{}
	if out, err := p.listUnits(ctx); err == nil {
		for _, line := range strings.Split(out, "\n") {
			// Columns (--plain --no-legend): UNIT LOAD ACTIVE SUB DESCRIPTION...
			f := strings.Fields(line)
			if len(f) < 4 {
				continue
			}
			state := "stopped"
			if f[2] == "active" {
				state = "running"
			}
			res = append(res, Resource{
				Type:  "object",
				Ref:   f[0],
				Name:  strings.TrimSuffix(f[0], ".service"),
				State: state,
				Attributes: map[string]string{
					"load":   f[1],
					"active": f[2],
					"sub":    f[3],
				},
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

func (p *systemdProvider) listUnits(ctx context.Context) (string, error) {
	return run(ctx, "systemctl", "list-units", "--type=service", "--all",
		"--no-legend", "--no-pager", "--plain")
}

func (p *systemdProvider) Ports(ctx context.Context, inst Instance) Result {
	socks := listeningSockets(ctx)
	owners := systemdOwnerMap(ctx)
	return portsResult(attachOwners(socks, owners, os.Getpid()))
}
