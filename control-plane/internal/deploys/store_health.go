package deploys

import (
	"context"
	"time"
)

// Queries backing the three things a run's outcome feeds: the project's health state,
// webhook de-duplication, and the sweep that closes out runs whose agent vanished.

// SetHealthState records where a project stands after a run finished. It is advisory
// state for operators, so a failure to write it never fails a deploy.
func (s *Store) SetHealthState(ctx context.Context, projectID, state string) error {
	_, err := s.pool.Exec(ctx,
		`update deploy_projects set health_state = $2 where id = $1`, projectID, state)
	return err
}

// ClaimWebhookDelivery records a provider delivery and reports whether this caller is
// the first to see it. A retried delivery (both GitHub and GitLab retry) returns
// false, and the caller answers the provider without starting a second deploy.
func (s *Store) ClaimWebhookDelivery(ctx context.Context, projectID, deliveryID, commitSHA string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		insert into deploy_webhook_deliveries (project_id, delivery_id, commit_sha)
		values ($1, $2, $3)
		on conflict (project_id, delivery_id) do nothing
	`, projectID, deliveryID, commitSHA)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// RecentRunForCommit reports whether this project already has a run for this commit
// that is either still going or was started moments ago. That covers the case one
// delivery id cannot: a repo with both the push webhook and the generated CI job
// reports the same commit twice, through two different paths, within a second or two.
func (s *Store) RecentRunForCommit(ctx context.Context, projectID, commitSHA string, window time.Duration) (bool, error) {
	if commitSHA == "" {
		return false, nil
	}
	var exists bool
	err := s.pool.QueryRow(ctx, `
		select exists (
		  select 1 from deploy_runs
		  where project_id = $1
		    and commit_sha = $2
		    and (status in ('pending','running') or created_at > now() - $3::interval)
		)
	`, projectID, commitSHA, window.String()).Scan(&exists)
	return exists, err
}

// ActiveRun reports whether the project has a run in flight, so a second trigger can
// be skipped rather than racing it into the same clone path.
func (s *Store) ActiveRun(ctx context.Context, projectID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		select exists (
		  select 1 from deploy_runs
		  where project_id = $1 and status in ('pending','running')
		)
	`, projectID).Scan(&exists)
	return exists, err
}

// StuckRuns lists runs still marked running past their budget. The agent enforces its
// own timeout, but an agent that is killed or loses its host mid-deploy never reports
// anything, and the run would otherwise sit "running" forever.
func (s *Store) StuckRuns(ctx context.Context, grace time.Duration) ([]Run, error) {
	rows, err := s.pool.Query(ctx, `
		select `+runCols+`
		from deploy_runs r
		where r.status in ('pending','running')
		  and r.created_at < now() - (
		        coalesce(nullif((select p.deploy_timeout_seconds from deploy_projects p where p.id = r.project_id), 0), 900)
		        + $1
		      ) * interval '1 second'
		order by r.created_at asc
		limit 50
	`, int(grace.Seconds()))
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
