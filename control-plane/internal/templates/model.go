// Package templates serves the job template library: the built-in starting points
// that ship with CronCompose, plus whatever the team saves from their own jobs.
//
// A template is not a live object. Creating a job from one copies the script in and
// then the two have nothing to do with each other, which is deliberate: an operator
// editing a job should never wonder whether they are also editing six other jobs.
package templates

import "time"

// Template is one starting point for a new job.
type Template struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	Category     string            `json:"category"`
	Interpreter  string            `json:"interpreter"`
	ScriptBody   string            `json:"script_body"`
	ScheduleCron string            `json:"schedule_cron"`
	Timezone     string            `json:"timezone"`
	Env          map[string]string `json:"env"`
	// Builtin templates ship with the product and cannot be edited or deleted, so an
	// upgrade can replace them without stepping on anybody's saved work.
	Builtin   bool      `json:"builtin"`
	CreatedBy *string   `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateInput is the body of POST /job-templates. Usually sent by "save this job as a
// template" rather than typed by hand.
type CreateInput struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Category     string            `json:"category"`
	Interpreter  string            `json:"interpreter"`
	ScriptBody   string            `json:"script_body"`
	ScheduleCron string            `json:"schedule_cron"`
	Timezone     string            `json:"timezone"`
	Env          map[string]string `json:"env"`
	// FromJobID populates the fields above from an existing job when set, so the UI
	// can offer a one-click "save as template" without resending the script.
	FromJobID string `json:"from_job_id"`
}
