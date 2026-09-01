package connectors

import (
	"context"
	"os"
	"path/filepath"
)

type nginxProvider struct{}

func (nginxProvider) Kind() string { return "nginx" }

func (p *nginxProvider) Detect(ctx context.Context) []Instance {
	if !has("nginx") {
		return nil
	}
	// `nginx -v` prints to stderr and exits 0; run() captures combined output.
	out, _ := run(ctx, "nginx", "-v")

	prefix := nginxPrefix()
	paths := nginxConfigPaths(prefix)
	main := nginxMainConfig(prefix)

	// Editing needs write access to the config tree; an unknown prefix means there is
	// nothing this connector is allowed to touch, whatever privileges the agent holds.
	canEdit := prefix != "" && (writable(main) || os.Geteuid() == 0 || canSudo(ctx, "install"))
	// Reloading is a separate privilege from writing the files: an agent can easily
	// have one and not the other.
	canReload := canReloadNginx(ctx)
	// Start/stop/restart go through the service manager. On a host with no systemd
	// there is no service to drive, even though a reload via nginx's own signal
	// handling still works, so the two capabilities are reported separately.
	hasSystemd := systemdAvailable()
	canLifecycle := hasSystemd && canReload

	detail := map[string]string{}
	if prefix != "" {
		detail["config_prefix"] = prefix
	} else {
		detail["config"] = "nginx is installed but no nginx.conf was found in a known location; " +
			"config operations are unavailable"
	}
	if !hasSystemd {
		detail["lifecycle"] = "no systemd on this host; start/stop/restart are unavailable, " +
			"reload uses nginx -s reload"
	}

	return []Instance{{
		Kind:        "nginx",
		Version:     parseVersionAfterSlash(out),
		Status:      serviceStatus(ctx, "nginx"),
		ConfigPaths: paths,
		Manageable:  canEdit || canReload,
		Caps: Capabilities{
			ManagesConfig: prefix != "",
			CanValidate:   true,
			CanReload:     canReload,
			CanLifecycle:  canLifecycle,
			CanEdit:       canEdit,
		},
		Detail: detail,
	}}
}

// canReloadNginx reports whether this agent can make nginx re-read its config, by
// either route. systemctl is not required: `nginx -s reload` works on any host where
// the agent is root or has the sudoers grant for the nginx binary.
func canReloadNginx(ctx context.Context) bool {
	if os.Geteuid() == 0 {
		return true
	}
	return canSudo(ctx, "systemctl") || canSudo(ctx, "nginx")
}

func (p *nginxProvider) Resources(ctx context.Context, inst Instance) []Resource {
	out := []Resource{}
	seen := map[string]bool{}
	for _, base := range inst.ConfigPaths {
		for _, f := range confFiles(base) {
			if seen[f] {
				continue
			}
			seen[f] = true
			sum, size := fileChecksum(f)
			out = append(out, Resource{
				Type:       "config_file",
				Ref:        f,
				Name:       filepath.Base(f),
				Checksum:   sum,
				SizeBytes:  size,
				Attributes: map[string]string{},
			})
		}
	}
	return out
}
