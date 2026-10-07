package deploys

import "testing"

func TestClonePathForUser(t *testing.T) {
	cases := []struct {
		path, runAs, home, want string
	}{
		{"/opt/apps/node/college-connect", "", "/home/pi", "/opt/apps/node/college-connect"},
		{"/opt/apps/node/college-connect", "root", "/root", "/opt/apps/node/college-connect"},
		{"/opt/apps/node/college-connect", "pi", "/home/pi", "/home/pi/opt/apps/node/college-connect"},
		{"/home/pi/opt/apps/node/x", "pi", "/home/pi", "/home/pi/opt/apps/node/x"},
		{"/var/apps/x", "pi", "/home/pi", "/home/pi/opt/apps/x"},
	}
	for _, c := range cases {
		got := ClonePathForUser(c.path, c.runAs, c.home)
		if got != c.want {
			t.Errorf("ClonePathForUser(%q,%q,%q)=%q want %q", c.path, c.runAs, c.home, got, c.want)
		}
	}
}

func TestUserTmpDir(t *testing.T) {
	if got := UserTmpDir("pi", "/home/pi"); got != "/home/pi/tmp" {
		t.Fatalf("got %q", got)
	}
	if got := UserTmpDir("root", "/root"); got != "/tmp" {
		t.Fatalf("got %q", got)
	}
}
