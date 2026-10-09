package connectors

import (
	"testing"
)

func TestValidActionIncludesPm2Extras(t *testing.T) {
	for _, a := range []string{"save", "startup", "delete", "flush", "start", "stop"} {
		if !ValidAction(a) {
			t.Fatalf("expected %q to be allowed", a)
		}
	}
}
