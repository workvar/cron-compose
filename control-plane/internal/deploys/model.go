package deploys

import "time"

// Project is a git repo deployed onto one agent server.
type Project struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Provider       string            `json:"provider"`
	RepoFullName   string            `json:"repo_full_name"`
	RepoID         string            `json:"repo_id,omitempty"`
	CloneURL       string            `json:"clone_url"`
	DefaultBranch  string            `json:"default_branch"`
	ServerID       string            `json:"server_id"`
	Language       string            `json:"language"`
	InstallScript  string            `json:"install_script"`
	RootDirectory  string            `json:"root_directory"`
	ClonePath      string            `json:"clone_path"`
	Port           int               `json:"port"`
	ProcessManager string            `json:"process_manager"`
	Env            map[string]string `json:"env"`
	Apps           []SpecApp         `json:"apps"`
	WriteSpec      bool              `json:"write_spec"`
	// AutoRollback, when true, redeploys the project's last successful commit
	// automatically after a failed run, and restarts the process manager on it.
	AutoRollback bool `json:"auto_rollback"`
	// Health is the opt-in "did it actually come up" probe. With HealthPath empty a
	// run succeeds as soon as the install script exits 0, which is what every project
	// did before this existed.
	HealthPath           string `json:"health_path"`
	HealthPort           int    `json:"health_port"`
	HealthTimeoutSeconds int    `json:"health_timeout_seconds"`
	// DeployTimeoutSeconds bounds a whole run. 0 means the agent's default.
	DeployTimeoutSeconds int `json:"deploy_timeout_seconds"`
	// HealthState is where the project stands now, as opposed to what its last run
	// did: unknown, healthy, degraded, or rolled_back.
	HealthState string    `json:"health_state"`
	CreatedBy   *string   `json:"created_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	DeployToken string    `json:"deploy_token,omitempty"` // only on create
}

// Project health states.
const (
	HealthUnknown    = "unknown"
	HealthHealthy    = "healthy"
	HealthDegraded   = "degraded"
	HealthRolledBack = "rolled_back"
)

// Run is one clone+install attempt.
type Run struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	ServerID  string `json:"server_id"`
	Trigger   string `json:"trigger"`
	Status    string `json:"status"`
	Branch    string `json:"branch"`
	// CommitSha is filled in once the agent reports the commit it checked out (see
	// agent/internal/deploy/runner.go); empty until then, and always empty for a run
	// that never got that far (agent_offline).
	CommitSha  string     `json:"commit_sha,omitempty"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	Error      string     `json:"error,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// LogLine is one chunk of a deploy run.
type LogLine struct {
	Stream string    `json:"stream"`
	Seq    int       `json:"seq"`
	Chunk  string    `json:"chunk"`
	Ts     time.Time `json:"ts"`
}

// Settings is the singleton language-path map.
type Settings struct {
	LanguagePaths map[string]string `json:"language_paths"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// MaxDirEntries is the cap for recursive directory listings.
const MaxDirEntries = 2000

// DirEntry is one directory in a listing.
type DirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// DirList is GET /git/dirs.
type DirList struct {
	Path      string     `json:"path"`
	Recursive bool       `json:"recursive"`
	Truncated bool       `json:"truncated"`
	Items     []DirEntry `json:"items"`
}

// Repo is a provider repository the user can import.
type Repo struct {
	ID            string `json:"id"`
	FullName      string `json:"full_name"`
	Description   string `json:"description,omitempty"`
	DefaultBranch string `json:"default_branch"`
	CloneURL      string `json:"clone_url"`
	Private       bool   `json:"private"`
}

// CreateInput is POST /deploys.
type CreateInput struct {
	Name           string            `json:"name"`
	Provider       string            `json:"provider"`
	RepoFullName   string            `json:"repo_full_name"`
	RepoID         string            `json:"repo_id,omitempty"`
	CloneURL       string            `json:"clone_url,omitempty"`
	DefaultBranch  string            `json:"default_branch,omitempty"`
	ServerID       string            `json:"server_id"`
	Language       string            `json:"language,omitempty"`
	InstallScript  string            `json:"install_script,omitempty"`
	RootDirectory  string            `json:"root_directory,omitempty"`
	ClonePath      string            `json:"clone_path,omitempty"`
	Port           int               `json:"port,omitempty"`
	ProcessManager string            `json:"process_manager,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Apps           []SpecApp         `json:"apps,omitempty"`
	WriteSpec      *bool             `json:"write_spec,omitempty"`
}

// Inspect is GET /git/inspect.
type Inspect struct {
	Detection
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	ClonePath     string `json:"clone_path"`
	ProcessHint   string `json:"process_manager"`
}
