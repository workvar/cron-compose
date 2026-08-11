package connectors

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

type pm2Provider struct{}

func (pm2Provider) Kind() string { return "pm2" }

// pm2Proc is the slice of `pm2 jlist` output we care about.
type pm2Proc struct {
	Name  string `json:"name"`
	PmID  int    `json:"pm_id"`
	Pm2Env struct {
		Status string `json:"status"`
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
	procs := p.list(ctx)
	status := "stopped"
	if len(procs) > 0 {
		status = "running"
	}
	user := os.Getenv("USER")
	return []Instance{{
		Kind:        "pm2",
		Instance:    user,
		Version:     ver,
		Status:      status,
		ObjectCount: len(procs),
		Manageable:  true, // pm2 acts within the agent user's own daemon
		Caps: Capabilities{
			ManagesObjects: true,
			CanLifecycle:   true,
		},
		Detail: map[string]string{"user": user},
	}}
}

func (p *pm2Provider) Resources(ctx context.Context, inst Instance) []Resource {
	res := []Resource{}
	for _, pr := range p.list(ctx) {
		res = append(res, Resource{
			Type:       "object",
			Ref:        strconv.Itoa(pr.PmID),
			Name:       pr.Name,
			State:      pr.Pm2Env.Status,
			Attributes: map[string]string{},
		})
	}
	return res
}

// list runs `pm2 jlist` and parses the JSON array. pm2 may emit banner lines before the
// JSON, so we scan to the first '['.
func (p *pm2Provider) list(ctx context.Context) []pm2Proc {
	out, err := run(ctx, "pm2", "jlist")
	if err != nil {
		return nil
	}
	i := strings.IndexByte(out, '[')
	if i < 0 {
		return nil
	}
	var procs []pm2Proc
	if err := json.Unmarshal([]byte(out[i:]), &procs); err != nil {
		return nil
	}
	return procs
}
