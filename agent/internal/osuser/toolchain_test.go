package osuser

import (
	"strings"
	"testing"
)

func TestWrapScriptLoadsNvm(t *testing.T) {
	got := WrapScript("command -v npm")
	for _, needle := range []string{"NVM_DIR", "nvm.sh", "versions/node", "command -v npm"} {
		if !strings.Contains(got, needle) {
			t.Fatalf("WrapScript missing %q:\n%s", needle, got)
		}
	}
}
