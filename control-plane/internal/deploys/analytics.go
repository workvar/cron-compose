package deploys

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

// Analytics is the chart payload for a project (or the fleet summary).
type Analytics struct {
	RunsTotal       int             `json:"runs_total"`
	Succeeded       int             `json:"succeeded"`
	Failed          int             `json:"failed"`
	SuccessRate     int             `json:"success_rate"`
	AvgDurationMs   int64           `json:"avg_duration_ms"`
	AvgReleaseBytes int64           `json:"avg_release_bytes"`
	DurationTrend   []TrendPoint    `json:"duration_trend"`
	ReleaseTrend    []TrendPoint    `json:"release_trend"`
	PhaseAvgMs      []PhasePoint    `json:"phase_avg_ms"`
	ByProject       []ProjectRollup `json:"by_project,omitempty"`
	Recent          []AnalyticsRun  `json:"recent"`
}

type TrendPoint struct {
	Label string `json:"label"`
	Value int64  `json:"value"`
	At    string `json:"at,omitempty"`
}

type PhasePoint struct {
	Phase string `json:"phase"`
	Ms    int64  `json:"ms"`
}

type ProjectRollup struct {
	ProjectID   string `json:"project_id"`
	Name        string `json:"name"`
	Runs        int    `json:"runs"`
	SuccessRate int    `json:"success_rate"`
	AvgMs       int64  `json:"avg_duration_ms"`
	AvgBytes    int64  `json:"avg_release_bytes"`
}

type AnalyticsRun struct {
	ID           string           `json:"id"`
	ProjectID    string           `json:"project_id"`
	ProjectName  string           `json:"project_name,omitempty"`
	Status       string           `json:"status"`
	Branch       string           `json:"branch"`
	CommitSHA    string           `json:"commit_sha,omitempty"`
	DurationMs   int64            `json:"duration_ms"`
	ReleaseBytes int64            `json:"release_bytes,omitempty"`
	PhaseMs      map[string]int64 `json:"phase_ms,omitempty"`
	CreatedAt    string           `json:"created_at"`
}

// ProjectAnalytics builds charts for one project's recent runs.
func (s *Store) ProjectAnalytics(ctx context.Context, projectID string, limit int) (Analytics, error) {
	runs, err := s.ListRuns(ctx, projectID, limit)
	if err != nil {
		return Analytics{}, err
	}
	return buildAnalytics(runs, nil), nil
}

// FleetAnalytics builds combined charts across recent deploy runs.
func (s *Store) FleetAnalytics(ctx context.Context, limit int) (Analytics, error) {
	if limit <= 0 {
		limit = 80
	}
	rows, err := s.pool.Query(ctx, `
		select r.id, r.project_id, r.server_id, r.trigger, r.status, r.branch, coalesce(r.commit_sha,''),
		  r.exit_code, coalesce(r.error,''), coalesce(r.metrics::text, '{}'),
		  r.started_at, r.finished_at, r.created_at, coalesce(p.name, '')
		from deploy_runs r
		left join deploy_projects p on p.id = r.project_id
		order by r.created_at desc
		limit $1
	`, limit)
	if err != nil {
		return Analytics{}, err
	}
	defer rows.Close()

	var runs []Run
	names := map[string]string{}
	for rows.Next() {
		var r Run
		var exit *int
		var metricsJSON, name string
		if err := rows.Scan(
			&r.ID, &r.ProjectID, &r.ServerID, &r.Trigger, &r.Status, &r.Branch, &r.CommitSha,
			&exit, &r.Error, &metricsJSON, &r.StartedAt, &r.FinishedAt, &r.CreatedAt, &name,
		); err != nil {
			return Analytics{}, err
		}
		r.ExitCode = exit
		if metricsJSON != "" && metricsJSON != "{}" {
			var m RunMetrics
			if json.Unmarshal([]byte(metricsJSON), &m) == nil {
				r.Metrics = &m
			}
		}
		runs = append(runs, r)
		if name != "" {
			names[r.ProjectID] = name
		}
	}
	if err := rows.Err(); err != nil {
		return Analytics{}, err
	}
	return buildAnalytics(runs, names), nil
}

func buildAnalytics(runs []Run, names map[string]string) Analytics {
	out := Analytics{
		DurationTrend: []TrendPoint{},
		ReleaseTrend:  []TrendPoint{},
		PhaseAvgMs:    []PhasePoint{},
		Recent:        []AnalyticsRun{},
	}
	if len(runs) == 0 {
		return out
	}

	phaseSum := map[string]int64{}
	phaseN := map[string]int{}
	var durSum, durN, bytesSum, bytesN int64
	type agg struct {
		name   string
		runs   int
		ok     int
		durSum int64
		durN   int64
		bSum   int64
		bN     int64
	}
	byProj := map[string]*agg{}

	// Oldest → newest for trends.
	chronological := append([]Run(nil), runs...)
	sort.Slice(chronological, func(i, j int) bool {
		return chronological[i].CreatedAt.Before(chronological[j].CreatedAt)
	})

	for _, r := range chronological {
		out.RunsTotal++
		switch r.Status {
		case "succeeded":
			out.Succeeded++
		case "failed", "canceled", "agent_offline":
			out.Failed++
		}
		ms := runDurationMs(r)
		label := shortRunLabel(r)
		if ms > 0 {
			out.DurationTrend = append(out.DurationTrend, TrendPoint{Label: label, Value: ms, At: r.CreatedAt.Format(time.RFC3339)})
			durSum += ms
			durN++
		}
		var releaseBytes int64
		if r.Metrics != nil {
			releaseBytes = r.Metrics.ReleaseBytes
			if releaseBytes > 0 {
				out.ReleaseTrend = append(out.ReleaseTrend, TrendPoint{Label: label, Value: releaseBytes, At: r.CreatedAt.Format(time.RFC3339)})
				bytesSum += releaseBytes
				bytesN++
			}
			for phase, pms := range r.Metrics.PhaseMs {
				if pms <= 0 {
					continue
				}
				phaseSum[phase] += pms
				phaseN[phase]++
			}
		}

		a := byProj[r.ProjectID]
		if a == nil {
			a = &agg{name: names[r.ProjectID]}
			byProj[r.ProjectID] = a
		}
		a.runs++
		if r.Status == "succeeded" {
			a.ok++
		}
		if ms > 0 {
			a.durSum += ms
			a.durN++
		}
		if releaseBytes > 0 {
			a.bSum += releaseBytes
			a.bN++
		}
	}

	if out.RunsTotal > 0 {
		out.SuccessRate = (out.Succeeded * 100) / out.RunsTotal
	}
	if durN > 0 {
		out.AvgDurationMs = durSum / durN
	}
	if bytesN > 0 {
		out.AvgReleaseBytes = bytesSum / bytesN
	}

	phaseOrder := []string{"preflight", "clone", "install", "pre_start", "release", "start", "health"}
	seen := map[string]bool{}
	for _, p := range phaseOrder {
		if n := phaseN[p]; n > 0 {
			out.PhaseAvgMs = append(out.PhaseAvgMs, PhasePoint{Phase: p, Ms: phaseSum[p] / int64(n)})
			seen[p] = true
		}
	}
	var extras []string
	for p := range phaseN {
		if !seen[p] {
			extras = append(extras, p)
		}
	}
	sort.Strings(extras)
	for _, p := range extras {
		out.PhaseAvgMs = append(out.PhaseAvgMs, PhasePoint{Phase: p, Ms: phaseSum[p] / int64(phaseN[p])})
	}

	if names != nil {
		for id, a := range byProj {
			rate := 0
			if a.runs > 0 {
				rate = (a.ok * 100) / a.runs
			}
			var avgMs, avgB int64
			if a.durN > 0 {
				avgMs = a.durSum / a.durN
			}
			if a.bN > 0 {
				avgB = a.bSum / a.bN
			}
			out.ByProject = append(out.ByProject, ProjectRollup{
				ProjectID: id, Name: a.name, Runs: a.runs, SuccessRate: rate, AvgMs: avgMs, AvgBytes: avgB,
			})
		}
		sort.Slice(out.ByProject, func(i, j int) bool {
			return out.ByProject[i].Runs > out.ByProject[j].Runs
		})
	}

	// Newest first for the recent table (cap 12).
	for i, r := range runs {
		if i >= 12 {
			break
		}
		ar := AnalyticsRun{
			ID: r.ID, ProjectID: r.ProjectID, ProjectName: names[r.ProjectID],
			Status: r.Status, Branch: r.Branch, CommitSHA: r.CommitSha,
			DurationMs: runDurationMs(r), CreatedAt: r.CreatedAt.Format(time.RFC3339),
		}
		if r.Metrics != nil {
			ar.ReleaseBytes = r.Metrics.ReleaseBytes
			ar.PhaseMs = r.Metrics.PhaseMs
		}
		out.Recent = append(out.Recent, ar)
	}
	return out
}

func runDurationMs(r Run) int64 {
	if r.StartedAt == nil || r.FinishedAt == nil {
		return 0
	}
	ms := r.FinishedAt.Sub(*r.StartedAt).Milliseconds()
	if ms < 0 {
		return 0
	}
	return ms
}

func shortRunLabel(r Run) string {
	if r.CommitSha != "" && len(r.CommitSha) >= 7 {
		return r.CommitSha[:7]
	}
	if r.Branch != "" {
		return r.Branch
	}
	return r.CreatedAt.Format("01/02")
}
