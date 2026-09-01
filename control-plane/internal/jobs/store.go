package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/ids"
)

// ErrNotFound is returned when a job id doesn't exist.
var ErrNotFound = errors.New("job not found")

// Store is the data-access layer for jobs and job_versions.
type Store struct{ pool *pgxpool.Pool }

// NewStore wires a Store to a pgx pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const selectCols = `
  j.id, j.target_kind, j.server_id, coalesce(j.target_labels::text,'{}'),
  j.name, coalesce(j.description,''), j.interpreter,
  j.schedule_cron, j.timezone, j.enabled, j.timeout_seconds,
  j.concurrency_policy, j.catchup_policy, j.max_retries,
  coalesce(j.working_dir,''), coalesce(j.run_as_user,''),
  j.cpu_quota_percent, j.memory_max_mb, j.tasks_max, j.io_weight,
  j.current_version_id, jv.version_number, jv.script_body,
  coalesce(jv.env::text,'{}'), coalesce(jv.secret_refs::text,'[]'),
  j.created_at, j.updated_at
`

func scanJob(row pgx.Row) (Job, error) {
	var j Job
	var serverID *string
	var labelsJSON, envJSON, refsJSON string
	err := row.Scan(
		&j.ID, &j.TargetKind, &serverID, &labelsJSON,
		&j.Name, &j.Description, &j.Interpreter,
		&j.ScheduleCron, &j.Timezone, &j.Enabled, &j.TimeoutSeconds,
		&j.ConcurrencyPolicy, &j.CatchupPolicy, &j.MaxRetries,
		&j.WorkingDir, &j.RunAsUser,
		&j.CPUQuotaPercent, &j.MemoryMaxMB, &j.TasksMax, &j.IOWeight,
		&j.CurrentVersionID, &j.CurrentVersion, &j.ScriptBody,
		&envJSON, &refsJSON,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return j, err
	}
	j.ServerID = serverID
	j.TargetLabels = map[string]string{}
	_ = json.Unmarshal([]byte(labelsJSON), &j.TargetLabels)
	j.Env = map[string]string{}
	_ = json.Unmarshal([]byte(envJSON), &j.Env)
	j.SecretRefs = []string{}
	_ = json.Unmarshal([]byte(refsJSON), &j.SecretRefs)
	return j, nil
}

// List returns jobs for a server, or every job if serverID is empty.
// For label-targeted jobs, "for a server" means any job whose selector matches the
// server's labels (jsonb containment).
func (s *Store) List(ctx context.Context, serverID string) ([]Job, error) {
	q := `select ` + selectCols + `
		from jobs j join job_versions jv on jv.id = j.current_version_id`
	args := []any{}
	if serverID != "" {
		q += `
		where (j.target_kind = 'server' and j.server_id = $1)
		   or (j.target_kind = 'labels'
		       and j.target_labels <@ coalesce((select labels from servers where id = $1), '{}'::jsonb))`
		args = append(args, serverID)
	}
	q += ` order by j.created_at desc`

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// Get returns one job by id or ErrNotFound.
func (s *Store) Get(ctx context.Context, id string) (Job, error) {
	q := `select ` + selectCols + `
		from jobs j join job_versions jv on jv.id = j.current_version_id
		where j.id = $1`
	j, err := scanJob(s.pool.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	return j, err
}

// MatchingServerIDs resolves a label selector to the set of currently matching
// server_ids. For target_kind=server it returns just that server id.
func (s *Store) MatchingServerIDs(ctx context.Context, j Job) ([]string, error) {
	if j.TargetKind == "server" {
		if j.ServerID == nil {
			return nil, nil
		}
		return []string{*j.ServerID}, nil
	}
	labels, _ := json.Marshal(j.TargetLabels)
	rows, err := s.pool.Query(ctx, `
		select id from servers where labels @> $1::jsonb
	`, labels)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Insert creates the job row and version 1 in a single transaction.
func (s *Store) Insert(ctx context.Context, in CreateInput) (Job, error) {
	applyDefaults(&in)
	if err := validateTarget(in.TargetKind, in.ServerID, in.TargetLabels); err != nil {
		return Job{}, err
	}
	envJSON, _ := json.Marshal(in.Env)
	refsJSON, _ := json.Marshal(coalesceRefs(in.SecretRefs))

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback(ctx)

	jobID := ids.New()
	versionID := ids.New()

	if _, err := tx.Exec(ctx, `
		insert into jobs (
		  id, target_kind, server_id, target_labels,
		  name, description, interpreter,
		  schedule_cron, timezone, enabled, timeout_seconds,
		  concurrency_policy, catchup_policy, max_retries,
		  working_dir, run_as_user, cpu_quota_percent, memory_max_mb, tasks_max, io_weight,
		  current_version_id
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
	`,
		jobID, in.TargetKind, nullIfEmpty(in.ServerID), marshalLabels(in.TargetLabels),
		in.Name, in.Description, in.Interpreter,
		in.ScheduleCron, in.Timezone, *in.Enabled, in.TimeoutSeconds,
		in.ConcurrencyPolicy, in.CatchupPolicy, in.MaxRetries,
		nullIfEmpty(in.WorkingDir), nullIfEmpty(in.RunAsUser), in.CPUQuotaPercent, in.MemoryMaxMB, in.TasksMax, in.IOWeight,
		versionID,
	); err != nil {
		return Job{}, err
	}
	if _, err := tx.Exec(ctx, `
		insert into job_versions (id, job_id, version_number, script_body, env, secret_refs)
		values ($1, $2, 1, $3, $4, $5)
	`, versionID, jobID, in.ScriptBody, envJSON, refsJSON); err != nil {
		return Job{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, err
	}
	return s.Get(ctx, jobID)
}

// Patch applies fields from in. If ScriptBody, Env, or SecretRefs changed, a new
// version is inserted and current_version_id is bumped.
func (s *Store) Patch(ctx context.Context, id string, in PatchInput) (Job, error) {
	job, err := s.Get(ctx, id)
	if err != nil {
		return Job{}, err
	}

	if in.TargetKind != nil {
		job.TargetKind = *in.TargetKind
	}
	if in.ServerID != nil {
		v := *in.ServerID
		if v == "" {
			job.ServerID = nil
		} else {
			job.ServerID = &v
		}
	}
	if in.TargetLabels != nil {
		job.TargetLabels = *in.TargetLabels
	}
	if in.Name != nil {
		job.Name = *in.Name
	}
	if in.Description != nil {
		job.Description = *in.Description
	}
	if in.Interpreter != nil {
		job.Interpreter = *in.Interpreter
	}
	if in.ScheduleCron != nil {
		job.ScheduleCron = *in.ScheduleCron
	}
	if in.Timezone != nil {
		job.Timezone = *in.Timezone
	}
	if in.Enabled != nil {
		job.Enabled = *in.Enabled
	}
	if in.TimeoutSeconds != nil {
		job.TimeoutSeconds = *in.TimeoutSeconds
	}
	if in.ConcurrencyPolicy != nil {
		job.ConcurrencyPolicy = *in.ConcurrencyPolicy
	}
	if in.CatchupPolicy != nil {
		job.CatchupPolicy = *in.CatchupPolicy
	}
	if in.MaxRetries != nil {
		job.MaxRetries = *in.MaxRetries
	}
	if in.WorkingDir != nil {
		job.WorkingDir = *in.WorkingDir
	}
	if in.RunAsUser != nil {
		job.RunAsUser = *in.RunAsUser
	}
	if in.CPUQuotaPercent != nil {
		job.CPUQuotaPercent = *in.CPUQuotaPercent
	}
	if in.MemoryMaxMB != nil {
		job.MemoryMaxMB = *in.MemoryMaxMB
	}
	if in.TasksMax != nil {
		job.TasksMax = *in.TasksMax
	}
	if in.IOWeight != nil {
		job.IOWeight = *in.IOWeight
	}

	sidArg := ""
	if job.ServerID != nil {
		sidArg = *job.ServerID
	}
	if err := validateTarget(job.TargetKind, sidArg, job.TargetLabels); err != nil {
		return Job{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback(ctx)

	if in.ScriptBody != nil || in.Env != nil || in.SecretRefs != nil {
		newVersionID := ids.New()
		nextVer := job.CurrentVersion + 1
		newScript := job.ScriptBody
		if in.ScriptBody != nil {
			newScript = *in.ScriptBody
		}
		newEnv := job.Env
		if in.Env != nil {
			newEnv = *in.Env
		}
		newRefs := job.SecretRefs
		if in.SecretRefs != nil {
			newRefs = *in.SecretRefs
		}
		envJSON, _ := json.Marshal(newEnv)
		refsJSON, _ := json.Marshal(coalesceRefs(newRefs))
		if _, err := tx.Exec(ctx, `
			insert into job_versions (id, job_id, version_number, script_body, env, secret_refs)
			values ($1, $2, $3, $4, $5, $6)
		`, newVersionID, id, nextVer, newScript, envJSON, refsJSON); err != nil {
			return Job{}, err
		}
		job.CurrentVersionID = newVersionID
		job.CurrentVersion = nextVer
		job.ScriptBody = newScript
		job.Env = newEnv
		job.SecretRefs = newRefs
	}

	if _, err := tx.Exec(ctx, `
		update jobs set
		  target_kind = $1, server_id = $2, target_labels = $3,
		  name = $4, description = $5, interpreter = $6,
		  schedule_cron = $7, timezone = $8, enabled = $9, timeout_seconds = $10,
		  concurrency_policy = $11, catchup_policy = $12, max_retries = $13,
		  working_dir = $14, run_as_user = $15,
		  cpu_quota_percent = $16, memory_max_mb = $17,
		  tasks_max = $18, io_weight = $19,
		  current_version_id = $20, updated_at = now()
		where id = $21
	`,
		job.TargetKind, nullIfEmpty(sidArg), marshalLabels(job.TargetLabels),
		job.Name, job.Description, job.Interpreter,
		job.ScheduleCron, job.Timezone, job.Enabled, job.TimeoutSeconds,
		job.ConcurrencyPolicy, job.CatchupPolicy, job.MaxRetries,
		nullIfEmpty(job.WorkingDir), nullIfEmpty(job.RunAsUser),
		job.CPUQuotaPercent, job.MemoryMaxMB,
		job.TasksMax, job.IOWeight,
		job.CurrentVersionID,
		id,
	); err != nil {
		return Job{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, err
	}
	return s.Get(ctx, id)
}

// SetEnabled flips the enabled flag.
func (s *Store) SetEnabled(ctx context.Context, id string, enabled bool) error {
	tag, err := s.pool.Exec(ctx, `update jobs set enabled = $1, updated_at = now() where id = $2`, enabled, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a job (cascades to versions and runs).
func (s *Store) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `delete from jobs where id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// coalesceRefs ensures we always write a JSON array, never null.
func coalesceRefs(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
