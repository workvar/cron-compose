package connectors

import (
	"context"
	"encoding/json"
	"strconv"
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

type dockerInspectRow struct {
	ID    string `json:"Id"`
	Name  string `json:"Name"`
	State struct {
		Status string `json:"Status"`
	} `json:"State"`
	Config struct {
		Image      string   `json:"Image"`
		Cmd        []string `json:"Cmd"`
		Entrypoint []string `json:"Entrypoint"`
		Env        []string `json:"Env"`
		WorkingDir string   `json:"WorkingDir"`
		Labels     map[string]string `json:"Labels"`
	} `json:"Config"`
}

func (p *dockerProvider) Resources(ctx context.Context, inst Instance) []Resource {
	rows := p.inspectAll(ctx)
	if len(rows) == 0 {
		// Fallback to docker ps when inspect fails (daemon busy / permission).
		return p.resourcesFromPS(ctx)
	}
	res := []Resource{}
	for _, row := range rows {
		res = append(res, dockerObject(row))
		if len(res) >= 250 {
			break
		}
	}
	return res
}

func (p *dockerProvider) resourcesFromPS(ctx context.Context) []Resource {
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

func (p *dockerProvider) inspectAll(ctx context.Context) []dockerInspectRow {
	ids, err := run(ctx, "docker", "ps", "-aq")
	if err != nil || strings.TrimSpace(ids) == "" {
		return nil
	}
	idList := strings.Fields(ids)
	args := append([]string{"inspect"}, idList...)
	out, err := run(ctx, "docker", args...)
	if err != nil {
		return nil
	}
	var rows []dockerInspectRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return nil
	}
	return rows
}

func dockerObject(row dockerInspectRow) Resource {
	name := strings.TrimPrefix(row.Name, "/")
	if name == "" {
		name = shortDockerID(row.ID)
	}
	env := parseEnvList(row.Config.Env)
	cmd := strings.Join(row.Config.Cmd, " ")
	if len(row.Config.Entrypoint) > 0 {
		cmd = strings.TrimSpace(strings.Join(row.Config.Entrypoint, " ") + " " + cmd)
	}
	attrs := map[string]string{
		"image":     row.Config.Image,
		"command":   cmd,
		"cwd":       row.Config.WorkingDir,
		"env_count": strconv.Itoa(len(env)),
		"env_keys":  envKeysCSV(env),
		"status":    row.State.Status,
	}
	if row.Config.Labels != nil {
		if v := row.Config.Labels["com.docker.compose.project"]; v != "" {
			attrs["compose_project"] = v
		}
		if v := row.Config.Labels["com.docker.compose.service"]; v != "" {
			attrs["compose_service"] = v
		}
	}
	return Resource{
		Type:       "object",
		Ref:        shortDockerID(row.ID),
		Name:       name,
		State:      row.State.Status,
		Attributes: attrs,
	}
}

func shortDockerID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func (p *dockerProvider) find(ctx context.Context, ref string) (dockerInspectRow, bool) {
	out, err := run(ctx, "docker", "inspect", ref)
	if err != nil {
		return dockerInspectRow{}, false
	}
	var rows []dockerInspectRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) == 0 {
		return dockerInspectRow{}, false
	}
	return rows[0], true
}

// Inspect returns Cmd/Env/WorkingDir for one container.
func (p *dockerProvider) Inspect(ctx context.Context, inst Instance, ref string) Result {
	row, ok := p.find(ctx, ref)
	if !ok {
		return fail(StatusFailed, "docker container not found: "+ref)
	}
	name := strings.TrimPrefix(row.Name, "/")
	cmd := strings.Join(row.Config.Cmd, " ")
	if len(row.Config.Entrypoint) > 0 {
		cmd = strings.TrimSpace(strings.Join(row.Config.Entrypoint, " ") + " " + cmd)
	}
	extra := map[string]string{"image": row.Config.Image}
	if row.Config.Labels != nil {
		if v := row.Config.Labels["com.docker.compose.project"]; v != "" {
			extra["compose_project"] = v
		}
		if v := row.Config.Labels["com.docker.compose.service"]; v != "" {
			extra["compose_service"] = v
		}
	}
	return inspectJSON(ProcessDetail{
		Name:    name,
		Command: cmd,
		Cwd:     row.Config.WorkingDir,
		State:   row.State.Status,
		Env:     parseEnvList(row.Config.Env),
		Extra:   extra,
	})
}
