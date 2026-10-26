package procgroup

import "testing"

func TestCPURate(t *testing.T) {
	cases := []struct {
		quota, cpus int
		want        uint32
	}{
		{100, 1, 10000}, // one core of a one-core box is the whole machine
		{100, 4, 2500},
		{50, 8, 625},
		{400, 2, 10000}, // more cores than exist clamps to 100%
		{1, 64, 1},      // never zero, which Windows rejects
		{100, 0, 10000}, // bad cpu count treated as one
	}
	for _, c := range cases {
		if got := CPURate(c.quota, c.cpus); got != c.want {
			t.Errorf("CPURate(%d, %d) = %d, want %d", c.quota, c.cpus, got, c.want)
		}
	}
}

func TestLimitsAny(t *testing.T) {
	if (Limits{}).Any() {
		t.Fatal("zero limits should be none")
	}
	if !(Limits{TasksMax: 5}).Any() {
		t.Fatal("tasks cap should count")
	}
}
