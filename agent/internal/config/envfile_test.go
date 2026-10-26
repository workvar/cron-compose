package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	in := "\ufeff# comment\n\nTOKEN=abc\nexport A=\"quoted value\"\nB='x=y'\nbad line\n  C = spaced  \n"
	got := parseEnvFile(bufio.NewScanner(strings.NewReader(in)))
	want := map[string]string{"TOKEN": "abc", "A": "quoted value", "B": "x=y", "C": "spaced"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestLoadEnvFileDoesNotOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.env")
	if err := os.WriteFile(path, []byte("CC_TEST_KEEP=file\nCC_TEST_NEW=file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_ENV_FILE", path)
	t.Setenv("CC_TEST_KEEP", "process")
	if err := LoadEnvFile(); err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("CC_TEST_NEW")
	if os.Getenv("CC_TEST_KEEP") != "process" {
		t.Fatal("env file overrode a value already set")
	}
	if os.Getenv("CC_TEST_NEW") != "file" {
		t.Fatal("env file value not applied")
	}
}

func TestLoadEnvFileMissingIsFine(t *testing.T) {
	t.Setenv("AGENT_ENV_FILE", filepath.Join(t.TempDir(), "nope.env"))
	if err := LoadEnvFile(); err != nil {
		t.Fatal(err)
	}
}
