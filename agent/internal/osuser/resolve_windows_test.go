//go:build windows

package osuser

import "testing"

func TestResolveSelfAndEmptyNeedNoCredential(t *testing.T) {
	for _, name := range []string{"", "  "} {
		if c, err := Resolve(name); c != nil || err != nil {
			t.Fatalf("Resolve(%q) = %v, %v", name, c, err)
		}
	}
	users, err := ListUsers()
	if err != nil || len(users) != 1 {
		t.Fatalf("ListUsers = %v, %v", users, err)
	}
	if c, err := Resolve(users[0].Username); c != nil || err != nil {
		t.Fatalf("Resolve(self) = %v, %v", c, err)
	}
}

func TestShortName(t *testing.T) {
	if got := shortName(`WORKGROUP\deploy`); got != "deploy" {
		t.Fatalf("got %q", got)
	}
	if got := shortName("deploy"); got != "deploy" {
		t.Fatalf("got %q", got)
	}
}
