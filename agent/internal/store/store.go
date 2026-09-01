// Package store is the agent's local persistence. The skeleton uses a JSON file per
// kind so it has no cgo dependency; swap for SQLite (modernc.org/sqlite) or bbolt
// before Phase 1 ships.
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// JobDef is what the agent caches for each job. Mirrors proto JobDef but kept Go-native
// so the scheduler/executor can stay decoupled from gRPC types.
type JobDef struct {
	ID                string            `json:"id"`
	VersionID         string            `json:"version_id"`
	Name              string            `json:"name"`
	Interpreter       string            `json:"interpreter"`
	ScriptBody        string            `json:"script_body"`
	ScheduleCron      string            `json:"schedule_cron"`
	Timezone          string            `json:"timezone"`
	Enabled           bool              `json:"enabled"`
	TimeoutSeconds    int               `json:"timeout_seconds"`
	ConcurrencyPolicy string            `json:"concurrency_policy"`
	CatchupPolicy     string            `json:"catchup_policy"`
	MaxRetries        int               `json:"max_retries"`
	WorkingDir        string            `json:"working_dir"`
	RunAsUser         string            `json:"run_as_user"`
	Env               map[string]string `json:"env"`
	Secrets           map[string]string `json:"secrets"`
	CPUQuotaPercent   int               `json:"cpu_quota_percent"`
	MemoryMaxMB       int               `json:"memory_max_mb"`
	TasksMax          int               `json:"tasks_max"`
	IOWeight          int               `json:"io_weight"`
}

// Store is the on-disk cache of jobs the agent should run.
type Store struct {
	mu      sync.Mutex
	dataDir string
}

// New ensures the data dir exists and returns a Store rooted there.
func New(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dataDir: dataDir}, nil
}

// SaveJobs writes the full job set atomically.
func (s *Store) SaveJobs(jobs []JobDef) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tmp := filepath.Join(s.dataDir, "jobs.json.tmp")
	final := filepath.Join(s.dataDir, "jobs.json")
	b, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

// LoadJobs reads the cached job set, returning an empty slice if no file exists yet.
func (s *Store) LoadJobs() ([]JobDef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	final := filepath.Join(s.dataDir, "jobs.json")
	b, err := os.ReadFile(final)
	if errors.Is(err, os.ErrNotExist) {
		return []JobDef{}, nil
	}
	if err != nil {
		return nil, err
	}
	var jobs []JobDef
	if err := json.Unmarshal(b, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}
