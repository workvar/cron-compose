package templates

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/ids"
)

// ErrNotFound is returned when a template id does not exist.
var ErrNotFound = errors.New("template not found")

// ErrBuiltin is returned when someone tries to delete a shipped template.
var ErrBuiltin = errors.New("built-in templates cannot be modified")

// Store wraps the job_templates table.
type Store struct{ pool *pgxpool.Pool }

// NewStore wires a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const cols = `
  id, name, description, category, interpreter, script_body,
  schedule_cron, timezone, env, builtin, created_by, created_at
`

func scan(row pgx.Row) (Template, error) {
	var t Template
	var env []byte
	if err := row.Scan(&t.ID, &t.Name, &t.Description, &t.Category, &t.Interpreter,
		&t.ScriptBody, &t.ScheduleCron, &t.Timezone, &env, &t.Builtin,
		&t.CreatedBy, &t.CreatedAt); err != nil {
		return t, err
	}
	t.Env = map[string]string{}
	_ = json.Unmarshal(env, &t.Env)
	return t, nil
}

// List returns every template. Built-ins sort first inside each category, because the
// shipped ones are what a new user is looking for.
func (s *Store) List(ctx context.Context, category string) ([]Template, error) {
	rows, err := s.pool.Query(ctx, `
		select `+cols+` from job_templates
		 where ($1 = '' or category = $1)
		 order by category, builtin desc, name
	`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Template{}
	for rows.Next() {
		t, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Get returns one template.
func (s *Store) Get(ctx context.Context, id string) (Template, error) {
	t, err := scan(s.pool.QueryRow(ctx, `select `+cols+` from job_templates where id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Template{}, ErrNotFound
	}
	return t, err
}

// Insert saves a user template.
func (s *Store) Insert(ctx context.Context, in CreateInput, actor string) (Template, error) {
	id := ids.New()
	env, err := json.Marshal(orEmpty(in.Env))
	if err != nil {
		return Template{}, err
	}
	_, err = s.pool.Exec(ctx, `
		insert into job_templates
		  (id, name, description, category, interpreter, script_body,
		   schedule_cron, timezone, env, builtin, created_by)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,false,nullif($10,''))
	`, id, in.Name, in.Description, in.Category, in.Interpreter, in.ScriptBody,
		in.ScheduleCron, in.Timezone, env, actor)
	if err != nil {
		return Template{}, err
	}
	return s.Get(ctx, id)
}

// FromJob fills a CreateInput from an existing job's current version, so "save as
// template" does not require the client to resend the script it already rendered.
func (s *Store) FromJob(ctx context.Context, jobID string, in CreateInput) (CreateInput, error) {
	var env []byte
	err := s.pool.QueryRow(ctx, `
		select j.name, coalesce(j.description,''), j.interpreter, j.schedule_cron, j.timezone,
		       jv.script_body, coalesce(jv.env, '{}'::jsonb)
		  from jobs j
		  join job_versions jv on jv.id = j.current_version_id
		 where j.id = $1
	`, jobID).Scan(&in.Name, &in.Description, &in.Interpreter, &in.ScheduleCron,
		&in.Timezone, &in.ScriptBody, &env)
	if errors.Is(err, pgx.ErrNoRows) {
		return in, ErrNotFound
	}
	if err != nil {
		return in, err
	}
	in.Env = map[string]string{}
	_ = json.Unmarshal(env, &in.Env)
	return in, nil
}

// Delete removes a user template. Built-ins are refused.
func (s *Store) Delete(ctx context.Context, id string) error {
	var builtin bool
	err := s.pool.QueryRow(ctx, `select builtin from job_templates where id = $1`, id).Scan(&builtin)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if builtin {
		return ErrBuiltin
	}
	_, err = s.pool.Exec(ctx, `delete from job_templates where id = $1`, id)
	return err
}

func orEmpty(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}
