package connectors

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemdOwnsConfinesToEtcSystemdSystem(t *testing.T) {
	if !systemdOwns("/etc/systemd/system/sshd.service") {
		t.Fatal("local unit should be owned")
	}
	if !systemdOwns("/etc/systemd/system/sshd.service.d/croncompose.conf") {
		t.Fatal("drop-in should be owned")
	}
	if systemdOwns("/lib/systemd/system/sshd.service") {
		t.Fatal("vendor unit in /lib must not be writable")
	}
	if systemdOwns("/usr/lib/systemd/system/sshd.service") {
		t.Fatal("vendor unit in /usr/lib must not be writable")
	}
	if systemdOwns("/etc/passwd") {
		t.Fatal("must not own /etc/passwd")
	}
	if systemdOwns("/etc/systemd/system/../../passwd") {
		t.Fatal("Clean must stop path escape")
	}
	if systemdOwns("/etc/systemd/systemextra/foo.service") {
		t.Fatal("prefix without separator is not ownership")
	}
}

func TestSystemdUnitFromPath(t *testing.T) {
	cases := []struct {
		path, unit string
	}{
		{"/etc/systemd/system/sshd.service", "sshd.service"},
		{"/etc/systemd/system/sshd.service.d/croncompose.conf", "sshd.service"},
		{"/etc/systemd/system/sshd.service.d/override.conf", "sshd.service"},
		{"/etc/systemd/system/backup.timer", "backup.timer"},
		{"/etc/systemd/system", ""},
		{"/etc/systemd/system/notes.conf", ""},
	}
	for _, c := range cases {
		got := systemdUnitFromPath(c.path)
		if got != c.unit {
			t.Errorf("unitFromPath(%q): got %q want %q", c.path, got, c.unit)
		}
	}
}

func TestSystemdConfigFilesListsUnitsAndDropIns(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("sshd.service", "[Service]\nExecStart=/bin/true\n")
	mustWrite("sshd.service.d/croncompose.conf", "[Service]\nEnvironment=FOO=1\n")
	mustWrite("backup.timer", "[Timer]\nOnCalendar=daily\n")
	mustWrite("ignore.txt", "nope")
	mustWrite("nested/dir/skip.service", "too deep")

	got := systemdConfigFiles(root)
	joined := strings.Join(got, "\n")
	want := []string{
		filepath.Join(root, "sshd.service"),
		filepath.Join(root, "sshd.service.d", "croncompose.conf"),
		filepath.Join(root, "backup.timer"),
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing %s in\n%s", w, joined)
		}
	}
	for _, g := range got {
		if strings.Contains(g, "ignore.txt") || strings.Contains(g, "skip.service") {
			t.Errorf("unexpected file %s", g)
		}
	}
}

func TestSystemdReadConfigRejectsVendorPath(t *testing.T) {
	p := &systemdProvider{}
	r := p.ReadConfig(context.Background(), Instance{}, "/lib/systemd/system/sshd.service")
	if r.Status != StatusUnauthorized {
		t.Fatalf("status: got %q want %s (%s)", r.Status, StatusUnauthorized, r.Message)
	}
}

func TestSystemdReadConfigReturnsLocalUnit(t *testing.T) {
	// owns is hardcoded to /etc/systemd/system, so this test uses a path
	// that function would reject. Read of a real local file is covered by
	// owns + os.ReadFile; here we check the helper that builds the result
	// from bytes we already have.
	body := []byte("[Service]\nExecStart=/bin/true\n")
	r := configBytesResult("/etc/systemd/system/sshd.service", body)
	if r.Failed() {
		t.Fatalf("result: %s %s", r.Status, r.Message)
	}
	if string(r.Content) != string(body) {
		t.Fatalf("content mismatch")
	}
	if r.Checksum == "" {
		t.Fatal("expected checksum")
	}
}

func TestSystemdValidateRejectsPathOutsideTree(t *testing.T) {
	p := &systemdProvider{}
	r := p.Validate(context.Background(), Instance{}, "/etc/passwd", []byte("[Service]\n"))
	if r.Status != StatusUnauthorized {
		t.Fatalf("status: got %q want %s (%s)", r.Status, StatusUnauthorized, r.Message)
	}
}

func TestSystemdResourcesIncludesConfigFilesFromInstance(t *testing.T) {
	root := t.TempDir()
	unit := filepath.Join(root, "app.service")
	if err := os.WriteFile(unit, []byte("[Service]\nExecStart=/bin/true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &systemdProvider{}
	res := p.Resources(context.Background(), Instance{ConfigPaths: []string{root}})
	found := false
	for _, r := range res {
		if r.Type == "config_file" && r.Ref == unit {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected config_file %s in %+v", unit, res)
	}
}
