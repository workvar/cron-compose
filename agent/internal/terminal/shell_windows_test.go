//go:build windows

package terminal

import (
	"reflect"
	"testing"
)

func TestShellArgvPerShell(t *testing.T) {
	cases := []struct {
		shell   string
		command bool
		want    []string
	}{
		{`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`, false, []string{"-NoLogo"}},
		{"pwsh.exe", true, []string{"-NoLogo", "-NoProfile", "-Command", "dir"}},
		{"cmd.exe", true, []string{"/C", "dir"}},
		{`C:\Program Files\Git\bin\bash.exe`, true, []string{"-lc", "dir"}},
	}
	for _, c := range cases {
		if got := shellArgv(c.shell, c.command, "dir"); !reflect.DeepEqual(got, c.want) {
			t.Errorf("shellArgv(%q, %v) = %v, want %v", c.shell, c.command, got, c.want)
		}
	}
}

func TestEnvBlockIsDoubleNulTerminated(t *testing.T) {
	b := envBlock([]string{"A=1"})
	if len(b) < 2 || b[len(b)-1] != 0 || b[len(b)-2] != 0 {
		t.Fatalf("block not double-NUL terminated: %v", b)
	}
}
