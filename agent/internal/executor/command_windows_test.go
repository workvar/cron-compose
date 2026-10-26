//go:build windows

package executor

import (
	"os"
	"strings"
	"testing"
)

func TestWindowsCommandNativeShellsUseTempFiles(t *testing.T) {
	for _, interp := range []string{"powershell", "pwsh", "cmd"} {
		prog, _, file, err := windowsCommand(interp, "echo hi")
		if err != nil {
			t.Fatalf("%s: %v", interp, err)
		}
		if file == "" || prog == "" {
			t.Fatalf("%s: prog=%q file=%q", interp, prog, file)
		}
		os.Remove(file)
	}
}

func TestWindowsCommandNodeEvaluates(t *testing.T) {
	_, args, _, err := windowsCommand("node", "console.log(1)")
	if err != nil || args[0] != "-e" {
		t.Fatalf("args=%v err=%v", args, err)
	}
}

func TestRunPowerShellCapturesStdout(t *testing.T) {
	var out []string
	res := Run(t.Context(), Job{Interpreter: "powershell", ScriptBody: "Write-Output hello", TimeoutSeconds: 60},
		func(stream string, _ int, data []byte) {
			if stream == "stdout" {
				out = append(out, string(data))
			}
		})
	if res.Status != "succeeded" {
		t.Fatalf("status=%s err=%s", res.Status, res.Err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "hello") {
		t.Fatalf("stdout=%v", out)
	}
}
