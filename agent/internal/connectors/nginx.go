package connectors

import (
	"context"
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
	paths := existingPaths(
		"/etc/nginx/nginx.conf",
		"/etc/nginx/conf.d",
		"/etc/nginx/sites-enabled",
		"/etc/nginx/sites-available",
	)
	canEdit := writable("/etc/nginx/nginx.conf")
	return []Instance{{
		Kind:        "nginx",
		Version:     parseVersionAfterSlash(out),
		Status:      serviceStatus(ctx, "nginx"),
		ConfigPaths: paths,
		Manageable:  canEdit,
		Caps: Capabilities{
			ManagesConfig: true,
			CanValidate:   true,
			CanReload:     true,
			CanEdit:       canEdit,
		},
		Detail: map[string]string{},
	}}
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
