package hostmetrics

import (
	"testing"
)

func TestCollectReturnsBasics(t *testing.T) {
	s := Collect()
	if s.OS == "" {
		t.Fatal("expected os")
	}
	if s.CPUs < 1 {
		t.Fatalf("cpus=%d", s.CPUs)
	}
	if s.CollectedAt.IsZero() {
		t.Fatal("expected collected_at")
	}
	// On CI/dev hosts we should usually see memory + disk; soft-check only.
	if s.MemTotalBytes == 0 && s.DiskTotalBytes == 0 {
		t.Log("warning: no mem/disk samples on this platform")
	}
}

func TestPct(t *testing.T) {
	if got := pct(50, 100); got != 50 {
		t.Fatalf("got %v", got)
	}
	if got := pct(1, 0); got != 0 {
		t.Fatalf("got %v", got)
	}
}
