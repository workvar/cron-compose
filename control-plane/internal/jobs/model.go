// Package jobs owns the "job" entity: a scheduled shell script that targets either a
// single server or a label selector matching many servers.
package jobs

import "time"

// Job is the row in the jobs table plus the joined current_version.
type Job struct {
	ID                string            `json:"id"`
	TargetKind        string            `json:"target_kind"` // "server" | "labels"
	ServerID          *string           `json:"server_id,omitempty"`
	TargetLabels      map[string]string `json:"target_labels"`
	Name              string            `json:"name"`
	Description       string            `json:"description,omitempty"`
	Interpreter       string            `json:"interpreter"`
	ScheduleCron      string            `json:"schedule_cron"`
	Timezone          string            `json:"timezone"`
	Enabled           bool              `json:"enabled"`
	TimeoutSeconds    int               `json:"timeout_seconds"`
	ConcurrencyPolicy string            `json:"concurrency_policy"`
	CatchupPolicy     string            `json:"catchup_policy"`
	MaxRetries        int               `json:"max_retries"`
	WorkingDir        string            `json:"working_dir,omitempty"`
	RunAsUser         string            `json:"run_as_user,omitempty"`
	CPUQuotaPercent   int               `json:"cpu_quota_percent"`
	MemoryMaxMB       int               `json:"memory_max_mb"`
	TasksMax          int               `json:"tasks_max"`
	IOWeight          int               `json:"io_weight"`
	CurrentVersionID  string            `json:"current_version_id"`
	CurrentVersion    int               `json:"current_version"`
	ScriptBody        string            `json:"script_body"`
	Env               map[string]string `json:"env"`
	SecretRefs        []string          `json:"secret_refs"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// CreateInput is the body accepted by POST /jobs.
type CreateInput struct {
	TargetKind        string            `json:"target_kind,omitempty"` // defaults to "server"
	ServerID          string            `json:"server_id,omitempty"`
	TargetLabels      map[string]string `json:"target_labels,omitempty"`
	Name              string            `json:"name"`
	Description       string            `json:"description,omitempty"`
	Interpreter       string            `json:"interpreter,omitempty"`
	ScheduleCron      string            `json:"schedule_cron"`
	Timezone          string            `json:"timezone,omitempty"`
	Enabled           *bool             `json:"enabled,omitempty"`
	TimeoutSeconds    int               `json:"timeout_seconds,omitempty"`
	ConcurrencyPolicy string            `json:"concurrency_policy,omitempty"`
	CatchupPolicy     string            `json:"catchup_policy,omitempty"`
	MaxRetries        int               `json:"max_retries,omitempty"`
	WorkingDir        string            `json:"working_dir,omitempty"`
	RunAsUser         string            `json:"run_as_user,omitempty"`
	CPUQuotaPercent   int               `json:"cpu_quota_percent,omitempty"`
	MemoryMaxMB       int               `json:"memory_max_mb,omitempty"`
	TasksMax          int               `json:"tasks_max,omitempty"`
	IOWeight          int               `json:"io_weight,omitempty"`
	ScriptBody        string            `json:"script_body"`
	Env               map[string]string `json:"env,omitempty"`
	SecretRefs        []string          `json:"secret_refs,omitempty"`
}

// PatchInput allows partial edits.
type PatchInput struct {
	TargetKind        *string            `json:"target_kind,omitempty"`
	ServerID          *string            `json:"server_id,omitempty"`
	TargetLabels      *map[string]string `json:"target_labels,omitempty"`
	Name              *string            `json:"name,omitempty"`
	Description       *string            `json:"description,omitempty"`
	Interpreter       *string            `json:"interpreter,omitempty"`
	ScheduleCron      *string            `json:"schedule_cron,omitempty"`
	Timezone          *string            `json:"timezone,omitempty"`
	Enabled           *bool              `json:"enabled,omitempty"`
	TimeoutSeconds    *int               `json:"timeout_seconds,omitempty"`
	ConcurrencyPolicy *string            `json:"concurrency_policy,omitempty"`
	CatchupPolicy     *string            `json:"catchup_policy,omitempty"`
	MaxRetries        *int               `json:"max_retries,omitempty"`
	WorkingDir        *string            `json:"working_dir,omitempty"`
	RunAsUser         *string            `json:"run_as_user,omitempty"`
	CPUQuotaPercent   *int               `json:"cpu_quota_percent,omitempty"`
	MemoryMaxMB       *int               `json:"memory_max_mb,omitempty"`
	TasksMax          *int               `json:"tasks_max,omitempty"`
	IOWeight          *int               `json:"io_weight,omitempty"`
	ScriptBody        *string            `json:"script_body,omitempty"`
	Env               *map[string]string `json:"env,omitempty"`
	SecretRefs        *[]string          `json:"secret_refs,omitempty"`
}
