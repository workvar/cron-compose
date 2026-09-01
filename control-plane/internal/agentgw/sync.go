package agentgw

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/secrets"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// SecretResolver resolves secret names for a given job. Implemented by *secrets.Store.
type SecretResolver interface {
	ResolveForJob(ctx context.Context, jobID, serverID string, names []string) (map[string]string, error)
}

// BuildFullSync loads every job assigned to a server, resolves referenced secrets, and
// returns a full-snapshot SyncJobs. A job is assigned when either:
//   - target_kind = 'server' AND server_id = $1
//   - target_kind = 'labels' AND target_labels are contained in the server's labels.
func BuildFullSync(ctx context.Context, pool *pgxpool.Pool, resolver SecretResolver, log *slog.Logger, serverID string) (*agentv1.SyncJobs, error) {
	rows, err := pool.Query(ctx, `
		select j.id, jv.id, jv.version_number, j.name, j.interpreter, jv.script_body,
		       j.schedule_cron, j.timezone, j.enabled, j.timeout_seconds,
		       j.concurrency_policy, j.catchup_policy, j.max_retries,
		       coalesce(j.working_dir,''), coalesce(j.run_as_user,''),
		       coalesce(jv.env::text, '{}'),
		       coalesce(jv.secret_refs::text, '[]'),
		       j.cpu_quota_percent, j.memory_max_mb, j.tasks_max, j.io_weight
		from jobs j
		join job_versions jv on jv.id = j.current_version_id
		where (j.target_kind = 'server' and j.server_id = $1)
		   or (j.target_kind = 'labels'
		       and j.target_labels <@ coalesce((select labels from servers where id = $1), '{}'::jsonb))
	`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type loaded struct {
		def  *agentv1.JobDef
		refs []string
	}
	var rowsBuf []loaded
	for rows.Next() {
		var d agentv1.JobDef
		var envJSON, refsJSON string
		if err := rows.Scan(
			&d.Id, &d.VersionId, &d.VersionNumber, &d.Name, &d.Interpreter, &d.ScriptBody,
			&d.ScheduleCron, &d.Timezone, &d.Enabled, &d.TimeoutSeconds,
			&d.ConcurrencyPolicy, &d.CatchupPolicy, &d.MaxRetries,
			&d.WorkingDir, &d.RunAsUser, &envJSON, &refsJSON,
			&d.CpuQuotaPercent, &d.MemoryMaxMb, &d.TasksMax, &d.IoWeight,
		); err != nil {
			return nil, err
		}
		env := map[string]string{}
		_ = json.Unmarshal([]byte(envJSON), &env)
		d.Env = env

		refs := []string{}
		_ = json.Unmarshal([]byte(refsJSON), &refs)
		rowsBuf = append(rowsBuf, loaded{def: &d, refs: refs})
	}

	defs := make([]*agentv1.JobDef, 0, len(rowsBuf))
	for _, l := range rowsBuf {
		if resolver != nil && len(l.refs) > 0 {
			vals, err := resolver.ResolveForJob(ctx, l.def.GetId(), serverID, l.refs)
			if err != nil {
				if log != nil {
					log.Warn("resolve secrets failed", "job_id", l.def.GetId(), "err", err)
				}
			} else {
				l.def.Secrets = vals
			}
		}
		defs = append(defs, l.def)
	}

	return &agentv1.SyncJobs{
		Cursor:       time.Now().UTC().Format(time.RFC3339Nano),
		Upsert:       defs,
		FullSnapshot: true,
	}, nil
}

// PushFullSync sends a fresh SyncJobs to the named server.
func (g *Gateway) PushFullSync(ctx context.Context, serverID string) error {
	sync, err := BuildFullSync(ctx, g.pool, g.resolver, g.log, serverID)
	if err != nil {
		return err
	}
	return g.registry.Send(serverID, &agentv1.ServerMessage{
		Body: &agentv1.ServerMessage_SyncJobs{SyncJobs: sync},
	})
}

// PushRunNow sends a RunNow command to the agent.
func (g *Gateway) PushRunNow(serverID, jobID, jobVersionID, runID string) error {
	return g.registry.Send(serverID, &agentv1.ServerMessage{
		Body: &agentv1.ServerMessage_RunNow{RunNow: &agentv1.RunNow{
			JobId:        jobID,
			JobVersionId: jobVersionID,
			RunId:        runID,
		}},
	})
}

// Suppress unused-import lint when secrets type is referenced only via the interface.
var _ = (SecretResolver)((*secrets.Store)(nil))
