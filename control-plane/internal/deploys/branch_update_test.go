package deploys

import "testing"

func TestSameCommit(t *testing.T) {
	full := "abcdef0123456789abcdef0123456789abcdef01"
	if !sameCommit(full, full) {
		t.Fatal("identical")
	}
	if !sameCommit(full, full[:7]) {
		t.Fatal("prefix short")
	}
	if !sameCommit(full[:7], full) {
		t.Fatal("short prefix")
	}
	if sameCommit(full, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef") {
		t.Fatal("different")
	}
	if sameCommit("", full) || sameCommit(full, "") {
		t.Fatal("empty")
	}
}
