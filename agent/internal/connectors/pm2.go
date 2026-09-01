package connectors

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

type pm2Provider struct{}

func (pm2Provider) Kind() string { return "pm2" }

// pm2Proc is the slice of `pm2 jlist` output we care about.
type pm2Proc struct {
	Name   string `json:"name"`
	PmID   int    `json:"pm_id"`
	Pid    int    `json:"pid"`
	Pm2Env struct {
		Status      string `json:"status"`
		RestartTime int    `json:"restart_time"`
		ExecMode    string `json:"exec_mode"`
		ExecPath    string `json:"pm_exec_path"`
		Cwd         string `json:"pm_cwd"`
	} `json:"pm2_env"`
}

func (p *pm2Provider) Detect(ctx context.Context) []Instance {
	if !has("pm2") {
		return nil
	}
	ver := ""
	if out, err := run(ctx, "pm2", "--version"); err == nil {
		ver = lastLine(out)
	}
	status := "stopped"
	if _, err := run(ctx, "pm2", "ping"); err == nil {
		status = "running"
	}
	procs := p.list(ctx)
	user := os.Getenv("USER")
	dump := pm2DumpPath()
	paths := existingPaths(dump)
	canEdit := false
	if dump != "" {
		canEdit = writable(dump) || dirWritable(filepath.Dir(dump))
	}
	return []Instance{{
		Kind:        "pm2",
		Instance:    user,
		Version:     ver,
		Status:      status,
		ObjectCount: len(procs),
		ConfigPaths: paths,
		Manageable:  true, // pm2 acts within the agent user's own daemon
		Caps: Capabilities{
			ManagesObjects: true,
			ManagesConfig:  len(paths) > 0,
			CanValidate:    true,
			CanReload:      true,
			CanLifecycle:   true,
			CanEdit:        canEdit && len(paths) > 0,
		},
		Detail: map[string]string{"user": user},
	}}
}

func (p *pm2Provider) Resources(ctx context.Context, inst Instance) []Resource {
	res := []Resource{}
	for _, pr := range p.list(ctx) {
		res = append(res, pm2Object(pr))
		if len(res) >= 250 {
			break
		}
	}
	dump := ""
	if len(inst.ConfigPaths) > 0 {
		dump = inst.ConfigPaths[0]
	} else {
		dump = pm2DumpPath()
	}
	if dump != "" {
		if _, err := os.Stat(dump); err == nil {
			sum, size := fileChecksum(dump)
			res = append(res, Resource{
				Type:       "config_file",
				Ref:        dump,
				Name:       filepath.Base(dump),
				Checksum:   sum,
				SizeBytes:  size,
				Attributes: map[string]string{},
			})
		}
	}
	return res
}

func (p *pm2Provider) list(ctx context.Context) []pm2Proc {
	out, err := run(ctx, "pm2", "jlist")
	if err != nil {
		return nil
	}
	return parsePm2List(out)
}

// pm2DumpPath is the dump file `pm2 save` writes and `pm2 resurrect` reads.
func pm2DumpPath() string {
	home := os.Getenv("PM2_HOME")
	if home == "" {
		home = os.Getenv("HOME")
		if home == "" {
			return ""
		}
		home = filepath.Join(home, ".pm2")
	}
	return filepath.Join(home, "dump.pm2")
}

// parsePm2List decodes `pm2 jlist`. pm2 may emit banner lines before the JSON,
// including ones that themselves contain '[' (`[PM2] Spawning...`), so we try
// unmarshalling from each '[' until one succeeds.
func parsePm2List(out string) []pm2Proc {
	for i := 0; i < len(out); i++ {
		if out[i] != '[' {
			continue
		}
		var procs []pm2Proc
		if err := json.Unmarshal([]byte(out[i:]), &procs); err == nil {
			return procs
		}
	}
	return nil
}

func pm2Object(p pm2Proc) Resource {
	name := p.Name
	if name == "" {
		name = strconv.Itoa(p.PmID)
	}
	return Resource{
		Type:  "object",
		Ref:   strconv.Itoa(p.PmID),
		Name:  name,
		State: p.Pm2Env.Status,
		Attributes: map[string]string{
			"exec":     p.Pm2Env.ExecPath,
			"mode":     p.Pm2Env.ExecMode,
			"restarts": strconv.Itoa(p.Pm2Env.RestartTime),
			"cwd":      p.Pm2Env.Cwd,
		},
	}
}

func (p *pm2Provider) Ports(ctx context.Context, inst Instance) Result {
	owners := map[int]portOwner{}
	for _, pr := range p.list(ctx) {
		if pr.Pid <= 0 {
			continue
		}
		name := pr.Name
		if name == "" {
			name = strconv.Itoa(pr.PmID)
		}
		owners[pr.Pid] = portOwner{Ref: strconv.Itoa(pr.PmID), Name: name}
	}
	return portsResult(attachOwners(listeningSockets(ctx), owners, os.Getpid()))
}
