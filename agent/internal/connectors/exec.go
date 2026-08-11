package connectors

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"strings"
)

// run executes a command and returns its trimmed combined output. A non-nil error means
// the command failed or was not found; callers generally treat that as "unavailable".
func run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return strings.TrimSpace(buf.String()), err
}

// has reports whether a binary is on PATH.
func has(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// fileChecksum returns the sha256 hex and byte size of a file, best-effort. Either may be
// zero-valued if the agent cannot stat or read the file.
func fileChecksum(path string) (sum string, size int64) {
	info, err := os.Stat(path)
	if err != nil {
		return "", 0
	}
	size = info.Size()
	b, err := os.ReadFile(path)
	if err != nil {
		return "", size
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), size
}
