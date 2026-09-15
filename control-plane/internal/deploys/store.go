package deploys

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/croncompose/croncompose/control-plane/internal/ids"
)

// ErrNotFound is a missing project or run.
var ErrNotFound = errors.New("not found")

// Store is the deploy data-access layer.
type Store struct{ pool *pgxpool.Pool }

// NewStore wires a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const projectCols = `
  id, name, provider, repo_full_name, repo_id, clone_url, default_branch, server_id,
  language, install_script, root_directory, clone_path, port, process_manager,
  coalesce(env::text,'{}'), coalesce(apps::text,'[]'), write_spec, auto_rollback,
  created_by, created_at, updated_at
`

func scanProject(row pgx.Row) (Project, error) {
	var p Project
	var envJSON, appsJSON string
	err := row.Scan(
		&p.ID, &p.Name, &p.Provider, &p.RepoFullName, &p.RepoID, &p.CloneURL, &p.DefaultBranch, &p.ServerID,
		&p.Language, &p.InstallScript, &p.RootDirectory, &p.ClonePath, &p.Port, &p.ProcessManager,
		&envJSON, &appsJSON, &p.WriteSpec, &p.AutoRollback, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return p, err
	}
	p.Env = map[string]string{}
	_ = json.Unmarshal([]byte(envJSON), &p.Env)
	p.Apps = []SpecApp{}
	_ = json.Unmarshal([]byte(appsJSON), &p.Apps)
	return p, nil
}

// List returns every deploy project, newest first.
func (s *Store) List(ctx context.Context) ([]Project, error) {
	rows, err := s.pool.Query(ctx, `select `+projectCols+` from deploy_projects order by created_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Get loads one project.
func (s *Store) Get(ctx context.Context, id string) (Project, error) {
	p, err := scanProject(s.pool.QueryRow(ctx, `select `+projectCols+` from deploy_projects where id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return p, err
}

// GetByRepo finds a project for webhook delivery.
func (s *Store) GetByRepo(ctx context.Context, provider, fullName string) (Project, string, error) {
	p, err := s.lookupRepo(ctx, provider, fullName)
	if err != nil {
		return Project{}, "", err
	}
	var secret string
	if err := s.pool.QueryRow(ctx, `select webhook_secret from deploy_projects where id = $1`, p.ID).Scan(&secret); err != nil {
		return p, "", err
	}
	return p, secret, nil
}

func (s *Store) lookupRepo(ctx context.Context, provider, fullName string) (Project, error) {
	p, err := scanProject(s.pool.QueryRow(ctx, `
		select `+projectCols+` from deploy_projects
		where provider = $1 and repo_full_name = $2
		order by created_at desc limit 1
	`, provider, fullName))
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return p, err
}

// WebhookSecret returns the HMAC secret for a project.
func (s *Store) WebhookSecret(ctx context.Context, id string) (string, error) {
	var secret string
	err := s.pool.QueryRow(ctx, `select webhook_secret from deploy_projects where id = $1`, id).Scan(&secret)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return secret, err
}

// TokenHash returns the hashed deploy token.
func (s *Store) TokenHash(ctx context.Context, id string) (string, error) {
	var hash string
	err := s.pool.QueryRow(ctx, `select deploy_token_hash from deploy_projects where id = $1`, id).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return hash, err
}

// Insert creates a project and returns the plaintext deploy token once.
func (s *Store) Insert(ctx context.Context, in CreateInput, actor string) (Project, error) {
	if in.Name == "" {
		in.Name = in.RepoFullName
	}
	if in.DefaultBranch == "" {
		in.DefaultBranch = "main"
	}
	if in.RootDirectory == "" {
		in.RootDirectory = "."
	}
	if in.ProcessManager == "" {
		in.ProcessManager = "none"
	}
	if in.Env == nil {
		in.Env = map[string]string{}
	}
	if in.Apps == nil {
		in.Apps = []SpecApp{}
	}
	writeSpec := true
	if in.WriteSpec != nil {
		writeSpec = *in.WriteSpec
	}
	envJSON, _ := json.Marshal(in.Env)
	appsJSON, _ := json.Marshal(in.Apps)
	token := "ccdep_" + ids.New()
	wh := ids.New()
	id := ids.New()
	return s.insertRow(ctx, id, in, actor, token, wh, writeSpec, envJSON, appsJSON)
}

func (s *Store) insertRow(ctx context.Context, id string, in CreateInput, actor, token, wh string, writeSpec bool, envJSON, appsJSON []byte) (Project, error) {
	var createdBy any
	if actor != "" {
		createdBy = actor
	}
	if in.CloneURL == "" {
		in.CloneURL = httpsCloneURL(in.Provider, in.RepoFullName)
	}
	_, err := s.pool.Exec(ctx, `
		insert into deploy_projects (
		  id, name, provider, repo_full_name, repo_id, clone_url, default_branch, server_id,
		  language, install_script, root_directory, clone_path, port, process_manager,
		  env, apps, webhook_secret, deploy_token_hash, write_spec, created_by
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
	`, id, in.Name, in.Provider, in.RepoFullName, in.RepoID, in.CloneURL, in.DefaultBranch, in.ServerID,
		in.Language, in.InstallScript, in.RootDirectory, in.ClonePath, in.Port, in.ProcessManager,
		envJSON, appsJSON, wh, hashToken(token), writeSpec, createdBy)
	if err != nil {
		return Project{}, err
	}
	p, err := s.Get(ctx, id)
	if err != nil {
		return p, err
	}
	p.DeployToken = token
	return p, nil
}

// Update writes PATCH fields onto an existing project.
func (s *Store) Update(ctx context.Context, id string, in UpdateInput) (Project, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return Project{}, err
	}
	p = applyUpdate(p, in)
	if p.Env == nil {
		p.Env = map[string]string{}
	}
	if p.Apps == nil {
		p.Apps = []SpecApp{}
	}
	envJSON, _ := json.Marshal(p.Env)
	appsJSON, _ := json.Marshal(p.Apps)
	_, err = s.pool.Exec(ctx, `
		update deploy_projects set
		  name=$2, default_branch=$3, server_id=$4, language=$5, install_script=$6,
		  root_directory=$7, clone_path=$8, port=$9, process_manager=$10,
		  env=$11, apps=$12, write_spec=$13, auto_rollback=$14, updated_at=now()
		where id=$1
	`, p.ID, p.Name, p.DefaultBranch, p.ServerID, p.Language, p.InstallScript,
		p.RootDirectory, p.ClonePath, p.Port, p.ProcessManager, envJSON, appsJSON, p.WriteSpec, p.AutoRollback)
	if err != nil {
		return Project{}, err
	}
	return s.Get(ctx, id)
}

// Delete removes a project and its runs.
func (s *Store) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `delete from deploy_projects where id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func hashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

func randomBytes(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// InsertRun records a new deploy run.
func (s *Store) InsertRun(ctx context.Context, projectID, serverID, trigger, branch string) (Run, error) {
	id := ids.New()
	_, err := s.pool.Exec(ctx, `
		insert into deploy_runs (id, project_id, server_id, trigger, status, branch)
		values ($1,$2,$3,$4,'pending',$5)
	`, id, projectID, serverID, trigger, branch)
	if err != nil {
		return Run{}, err
	}
	return s.GetRun(ctx, id)
}

const runCols = `
  id, project_id, server_id, trigger, status, branch, coalesce(commit_sha,''),
  exit_code, coalesce(error,''), started_at, finished_at, created_at
`

func scanRun(row pgx.Row) (Run, error) {
	var r Run
	var exit *int
	err := row.Scan(&r.ID, &r.ProjectID, &r.ServerID, &r.Trigger, &r.Status, &r.Branch, &r.CommitSha,
		&exit, &r.Error, &r.StartedAt, &r.FinishedAt, &r.CreatedAt)
	r.ExitCode = exit
	return r, err
}

// GetRun loads one run.
func (s *Store) GetRun(ctx context.Context, id string) (Run, error) {
	r, err := scanRun(s.pool.QueryRow(ctx, `select `+runCols+` from deploy_runs where id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	return r, err
}

// ListRuns lists runs for a project.
func (s *Store) ListRuns(ctx context.Context, projectID string, limit int) ([]Run, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		select `+runCols+`
		from deploy_runs where project_id = $1 order by created_at desc limit $2
	`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LastSucceededRun returns the most recent run that both succeeded and has a recorded
// commit, i.e. the commit an auto-rollback should return to. ErrNotFound means there is
// nothing to roll back to yet (the project has never had a clean run).
func (s *Store) LastSucceededRun(ctx context.Context, projectID string) (Run, error) {
	r, err := scanRun(s.pool.QueryRow(ctx, `
		select `+runCols+`
		from deploy_runs
		where project_id = $1 and status = 'succeeded' and commit_sha <> ''
		order by created_at desc limit 1
	`, projectID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	return r, err
}

// MarkRun updates status fields.
func (s *Store) MarkRun(ctx context.Context, id, status string, exit int32, errMsg string) error {
	_, err := s.pool.Exec(ctx, `
		update deploy_runs set
		  status = $2,
		  exit_code = $3,
		  error = nullif($4,''),
		  started_at = coalesce(started_at, now()),
		  finished_at = case when $2 in ('succeeded','failed','canceled') then now() else finished_at end
		where id = $1
	`, id, status, exit, errMsg)
	return err
}

// AppendLog stores a log chunk.
func (s *Store) AppendLog(ctx context.Context, runID, stream string, seq int, chunk string) error {
	_, err := s.pool.Exec(ctx, `
		insert into deploy_run_logs (run_id, stream, seq, chunk)
		values ($1,$2,$3,$4)
		on conflict (run_id, stream, seq) do nothing
	`, runID, stream, seq, chunk)
	return err
}

// Logs returns stored chunks.
func (s *Store) Logs(ctx context.Context, runID string) ([]LogLine, error) {
	rows, err := s.pool.Query(ctx, `
		select stream, seq, chunk, ts from deploy_run_logs where run_id = $1 order by seq
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LogLine{}
	for rows.Next() {
		var l LogLine
		if err := rows.Scan(&l.Stream, &l.Seq, &l.Chunk, &l.Ts); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// GetSettings loads language paths, filling defaults for missing keys.
func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	var raw string
	var updated time.Time
	err := s.pool.QueryRow(ctx, `select coalesce(language_paths::text,'{}'), updated_at from deploy_settings where id = 'default'`).Scan(&raw, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{LanguagePaths: DefaultLanguagePaths(), UpdatedAt: time.Now()}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	paths := DefaultLanguagePaths()
	custom := map[string]string{}
	_ = json.Unmarshal([]byte(raw), &custom)
	for k, v := range custom {
		if v != "" {
			paths[k] = v
		}
	}
	return Settings{LanguagePaths: paths, UpdatedAt: updated}, nil
}

// PutSettings replaces custom language paths.
func (s *Store) PutSettings(ctx context.Context, paths map[string]string) (Settings, error) {
	if paths == nil {
		paths = map[string]string{}
	}
	b, _ := json.Marshal(paths)
	_, err := s.pool.Exec(ctx, `
		insert into deploy_settings (id, language_paths, updated_at)
		values ('default', $1, now())
		on conflict (id) do update set language_paths = excluded.language_paths, updated_at = now()
	`, b)
	if err != nil {
		return Settings{}, err
	}
	return s.GetSettings(ctx)
}

func tokenOK(hash, plain string) bool {
	if hash == "" || plain == "" {
		return false
	}
	return hash == hashToken(plain)
}

// MatchToken reports whether plain matches the stored hash.
func MatchToken(hash, plain string) bool { return tokenOK(hash, plain) }
