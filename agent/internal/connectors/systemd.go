package connectors

import (
	"context"
	"os"
	"strings"
)

type systemdProvider struct{}

func (systemdProvider) Kind() string { return "systemd" }

func (p *systemdProvider) Detect(ctx context.Context) []Instance {
	if !has("systemctl") {
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
	// Lifecycle and unit-file edits need root (or a sudoers allowlist, a later phase).
	manageable := os.Geteuid() == 0
	return []Instance{{
		Kind:        "systemd",
		Version:     ver,
		Status:      "running",
		ObjectCount: count,
		Manageable:  manageable,
		Caps: Capabilities{
			ManagesObjects: true,
			ManagesConfig:  true,
			CanLifecycle:   manageable,
			CanEdit:        manageable,
		},
		Detail: map[string]string{},
	}}
}

func (p *systemdProvider) Resources(ctx context.Context, inst Instance) []Resource {
	out, err := p.listUnits(ctx)
	if err != nil {
		return nil
	}
	res := []Resource{}
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
	return res
}

func (p *systemdProvider) listUnits(ctx context.Context) (string, error) {
	return run(ctx, "systemctl", "list-units", "--type=service", "--all",
		"--no-legend", "--no-pager", "--plain")
}
