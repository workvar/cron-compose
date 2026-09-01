package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// JobState is the small amount of per-job history the agent has to survive a restart.
//
// It exists for catch-up: to know whether a scheduled window was missed while the
// agent was down, something has to remember when the job last actually ran. The
// control plane cannot answer that, because the whole point of agent-local scheduling
// is that jobs keep firing while the control plane is unreachable.
type JobState struct {
	LastFiredAt time.Time `json:"last_fired_at"`
}

// LoadState reads the per-job state map. A missing or corrupt file yields an empty
// map: losing this costs at most one skipped catch-up, so it is never worth failing
// startup over.
func (s *Store) LoadState() map[string]JobState {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(filepath.Join(s.dataDir, "state.json"))
	if errors.Is(err, os.ErrNotExist) || err != nil {
		return map[string]JobState{}
	}
	out := map[string]JobState{}
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]JobState{}
	}
	return out
}

// SaveState writes the per-job state map atomically.
func (s *Store) SaveState(state map[string]JobState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tmp := filepath.Join(s.dataDir, "state.json.tmp")
	final := filepath.Join(s.dataDir, "state.json")
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}
