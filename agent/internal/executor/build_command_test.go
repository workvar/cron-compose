//go:build !windows

package executor

import (
	"strings"
	"testing"
)

func TestBuildCommandNoLimits(t *testing.T) {
	prog, args := buildCommand("bash", Job{ScriptBody: "echo hi"})
	if prog != "bash" {
		t.Errorf("prog: got %q, want bash", prog)
	}
	if len(args) != 2 || args[0] != "-c" || args[1] != "echo hi" {
		t.Errorf("args: %#v", args)
	}
}

func TestBuildCommandWithLimitsWrapsWhenSystemdRunPresent(t *testing.T) {
	// We can't reliably assume systemd-run exists in the test sandbox, so we just
	// check the branching contract: if it WAS present and limits are set, the prog
	// would be systemd-run with the expected -p args BEFORE the interpreter.
	prog, args := buildCommand("bash", Job{
		ScriptBody:      "echo hi",
		CPUQuotaPercent: 50,
		MemoryMaxMB:     128,
	})
	if prog == "systemd-run" {
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "CPUQuota=50%") {
			t.Errorf("missing CPUQuota: %s", joined)
		}
		if !strings.Contains(joined, "MemoryMax=128M") {
			t.Errorf("missing MemoryMax: %s", joined)
		}
		// "-- bash -c echo hi" must come after the props.
		if !strings.Contains(joined, "-- bash -c") {
			t.Errorf("missing separator + interpreter: %s", joined)
		}
		return
	}
	// systemd-run isn't on PATH: the executor must transparently fall back.
	if prog != "bash" {
		t.Errorf("fallback prog: got %q, want bash", prog)
	}
}
