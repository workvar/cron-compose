// Package selfupdate replaces the running agent binary with a newer one.
//
// The design assumes a supervisor (pm2, systemd, docker) will restart the process, so
// the agent never tries to exec itself: it swaps the file and exits cleanly. That is
// both simpler and safer than re-exec, because a process that cannot restart itself
// under its supervisor is a process the operator has to go fix by hand.
//
// The checksum is not optional. Downloading a binary over the network and running it
// as whatever the agent runs as is the single most dangerous thing this program does,
// and a hash the control plane pinned is what makes it a controlled action rather than
// a remote code execution primitive.
package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// maxBinarySize bounds the download so a wrong URL cannot fill the disk.
const maxBinarySize = 256 << 20 // 256 MiB

// Request is one update instruction from the control plane.
type Request struct {
	TargetVersion string
	DownloadURL   string
	SHA256        string
}

// Validate rejects an instruction we should not act on before any network call.
func (r Request) Validate(currentVersion string) error {
	switch {
	case r.TargetVersion == "":
		return errors.New("no target version")
	case r.TargetVersion == currentVersion:
		return fmt.Errorf("already running %s", currentVersion)
	case r.DownloadURL == "":
		return errors.New("no download url")
	case !strings.HasPrefix(r.DownloadURL, "https://"):
		// The control plane connection is mTLS; the binary download should be at
		// least TLS. Plain http here would undo that in one line.
		return errors.New("download url must be https")
	case len(r.SHA256) != 64:
		return errors.New("a full sha256 hex digest is required")
	}
	return nil
}

// Apply downloads, verifies, and swaps the running binary. It returns the path that
// was replaced. The caller is expected to exit soon after so the supervisor restarts
// the new binary.
//
// The previous binary is kept alongside as <name>.old: if the new one refuses to start,
// that file is the difference between a two-second fix and a trip to the machine.
func Apply(ctx context.Context, req Request, currentVersion string) (string, error) {
	if err := req.Validate(currentVersion); err != nil {
		return "", err
	}

	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate self: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return "", fmt.Errorf("resolve self: %w", err)
	}

	tmp, err := download(ctx, req, filepath.Dir(self))
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp) // no-op once renamed

	info, err := os.Stat(self)
	if err != nil {
		return "", err
	}
	if err := os.Chmod(tmp, info.Mode().Perm()); err != nil {
		return "", err
	}

	// Keep the old binary. On Unix the running process holds its inode open, so this
	// rename is safe while we are executing.
	backup := self + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(self, backup); err != nil {
		return "", fmt.Errorf("set aside current binary: %w", err)
	}
	if err := os.Rename(tmp, self); err != nil {
		// Put the old one back rather than leaving the agent with no binary at all.
		_ = os.Rename(backup, self)
		return "", fmt.Errorf("install new binary: %w", err)
	}
	return self, nil
}

// download fetches the binary into a temp file beside the target, verifying the digest
// as it streams. A mismatch removes the file before returning, so a failed update never
// leaves a half-trusted binary on disk.
func download(ctx context.Context, req Request, dir string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.DownloadURL, nil)
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("User-Agent", "croncompose-agent-selfupdate/1")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	f, err := os.CreateTemp(dir, ".croncompose-agent-*")
	if err != nil {
		return "", err
	}
	name := f.Name()

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxBinarySize))
	closeErr := f.Close()
	if err != nil {
		os.Remove(name)
		return "", err
	}
	if closeErr != nil {
		os.Remove(name)
		return "", closeErr
	}
	if n >= maxBinarySize {
		os.Remove(name)
		return "", fmt.Errorf("download exceeded %d bytes", int64(maxBinarySize))
	}

	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, req.SHA256) {
		os.Remove(name)
		return "", fmt.Errorf("checksum mismatch: got %s, expected %s", got, req.SHA256)
	}
	return name, nil
}
