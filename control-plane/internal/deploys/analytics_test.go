package deploys

import (
	"testing"
	"time"
)

func TestBuildAnalytics(t *testing.T) {
	now := time.Now()
	start := now.Add(-30 * time.Second)
	fin := now
	runs := []Run{
		{
			ID: "r1", ProjectID: "p1", Status: "succeeded", Branch: "main", CommitSha: "abcdef012345",
			StartedAt: &start, FinishedAt: &fin, CreatedAt: now.Add(-time.Hour),
			Metrics: &RunMetrics{
				PhaseMs:      map[string]int64{"clone": 2000, "install": 20000, "start": 1000},
				ReleaseBytes: 50 << 20,
			},
		},
		{
			ID: "r2", ProjectID: "p1", Status: "failed", Branch: "main", CommitSha: "bbbbbbb",
			StartedAt: &start, FinishedAt: &fin, CreatedAt: now,
			Metrics: &RunMetrics{PhaseMs: map[string]int64{"install": 10000}, ReleaseBytes: 40 << 20},
		},
	}
	a := buildAnalytics(runs, map[string]string{"p1": "shop"})
	if a.RunsTotal != 2 || a.Succeeded != 1 || a.Failed != 1 {
		t.Fatalf("counts = %+v", a)
	}
	if a.SuccessRate != 50 {
		t.Fatalf("success_rate=%d", a.SuccessRate)
	}
	if len(a.DurationTrend) != 2 || len(a.ReleaseTrend) != 2 {
		t.Fatalf("trends duration=%d release=%d", len(a.DurationTrend), len(a.ReleaseTrend))
	}
	if len(a.PhaseAvgMs) == 0 {
		t.Fatal("expected phase averages")
	}
	if len(a.ByProject) != 1 || a.ByProject[0].Name != "shop" {
		t.Fatalf("by_project=%+v", a.ByProject)
	}
}

func TestBuildAnalyticsEmpty(t *testing.T) {
	a := buildAnalytics(nil, nil)
	if a.RunsTotal != 0 || a.DurationTrend == nil {
		t.Fatalf("empty analytics should keep slices non-nil: %+v", a)
	}
}
