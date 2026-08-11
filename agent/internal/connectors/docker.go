package connectors

import (
	"context"
	"strings"
)

type dockerProvider struct{}

func (dockerProvider) Kind() string { return "docker" }

func (p *dockerProvider) Detect(ctx context.Context) []Instance {
	if !has("docker") {
		return nil
	}
	// `docker info` reaches the daemon; it succeeds when the agent can use Docker
	// (root, or a member of the docker group). That is the manageable signal.
	_, infoErr := run(ctx, "docker", "info", "--format", "{{.ServerVersion}}")
	manageable := infoErr == nil
	status := "running"
	if !manageable {
		status = "unknown"
	}
	ver := ""
	if out, err := run(ctx, "docker", "version", "--format", "{{.Server.Version}}"); err == nil {
		ver = strings.TrimSpace(out)
	}
	count := 0
	if out, err := run(ctx, "docker", "ps", "-a", "--format", "{{.ID}}"); err == nil {
		count = countLines(out)
	}
	return []Instance{{
		Kind:        "docker",
		Version:     ver,
		Status:      status,
		ObjectCount: count,
		Manageable:  manageable,
		Caps: Capabilities{
			ManagesObjects: true,
			CanLifecycle:   manageable,
		},
		Detail: map[string]string{},
	}}
}

func (p *dockerProvider) Resources(ctx context.Context, inst Instance) []Resource {
	out, err := run(ctx, "docker", "ps", "-a", "--format",
		"{{.ID}}\t{{.Names}}\t{{.State}}\t{{.Image}}\t{{.Status}}")
	if err != nil {
		return nil
	}
	res := []Resource{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 5 {
			continue
		}
		res = append(res, Resource{
			Type:  "object",
			Ref:   f[0],
			Name:  f[1],
			State: f[2],
			Attributes: map[string]string{
				"image":  f[3],
				"status": f[4],
			},
		})
		if len(res) >= 250 {
			break
		}
	}
	return res
}
