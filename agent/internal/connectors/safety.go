package connectors

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// applyConfig is the shared write pipeline every ConfigManager goes through. No
// provider writes a config file on its own; the ordering here is the safety property
// the whole connector feature rests on:
//
//  1. backup    read the current bytes so a rollback is always possible
//  2. precheck  refuse if the file changed under us since the operator's last read
//  3. validate  run the tool's own checker on the candidate, never on the live file
//  4. write     atomically replace the file (temp file + rename, so no torn config)
//     4b. validate-live  re-check the whole tree now the fragment is in place
//  5. activate  reload/restart the service
//  6. health    confirm the service is actually up afterwards
//
// If step 4b, 5 or 6 fails, the backup from step 1 is written back and the service is
// activated again, so a bad config never outlives the request that caused it. A
// dry run stops after step 3.
func applyConfig(ctx context.Context, p ConfigManager, inst Instance, cmd Command) Result {
	var steps []Step

	// 1. Backup.
	before, readErr := os.ReadFile(cmd.Ref)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fail(StatusFailed, "cannot read current config: "+readErr.Error(),
			step("backup", false, readErr.Error()))
	}
	existed := readErr == nil
	steps = append(steps, step("backup", true,
		fmt.Sprintf("%d bytes, checksum %s", len(before), shortSum(before))))

	// 2. Optimistic concurrency: the operator edited what they last read.
	if cmd.BaseChecksum != "" && existed {
		if got := checksum(before); got != cmd.BaseChecksum {
			return fail(StatusInvalid,
				"the file changed on the server since it was read; reload and re-apply",
				append(steps, step("precheck", false, "checksum "+got+" != "+cmd.BaseChecksum))...)
		}
		steps = append(steps, step("precheck", true, "checksum matches"))
	}

	// 3. Validate the candidate.
	v := p.Validate(ctx, inst, cmd.Ref, cmd.Content)
	steps = append(steps, step("validate", !v.Failed(), v.Message))
	if v.Failed() {
		return fail(v.Status, v.Message, steps...)
	}
	if cmd.DryRun {
		return Result{Status: StatusSucceeded, Message: "validated; not applied (dry run)", Steps: steps}
	}

	// 4. Atomic write.
	if err := writeFileAtomic(ctx, cmd.Ref, cmd.Content); err != nil {
		return fail(statusForWrite(err), "write failed: "+err.Error(),
			append(steps, step("write", false, err.Error()))...)
	}
	steps = append(steps, step("write", true, fmt.Sprintf("%d bytes", len(cmd.Content))))

	// 4b. Validate in context. A fragment can be individually well-formed and still
	// break the tree it is included into, so this is the check that actually decides.
	lv := p.ValidateLive(ctx, inst, cmd.Ref)
	steps = append(steps, step("validate-live", !lv.Failed(), lv.Message))
	if lv.Failed() {
		return rollback(ctx, p, inst, cmd.Ref, before, existed,
			fail(StatusInvalid, "config is invalid in context: "+lv.Message), steps)
	}

	// 5. Activate, 6. health gate. Either failing rolls back.
	act := p.Activate(ctx, inst, cmd.Ref)
	steps = append(steps, step("activate", !act.Failed(), act.Message))
	if act.Failed() {
		return rollback(ctx, p, inst, cmd.Ref, before, existed, act, steps)
	}

	if healthy, detail := serviceHealthy(ctx, inst); !healthy {
		steps = append(steps, step("health", false, detail))
		return rollback(ctx, p, inst, cmd.Ref, before, existed,
			fail(StatusFailed, "service did not come back healthy: "+detail), steps)
	}
	steps = append(steps, step("health", true, "service is running"))

	return Result{
		Status:   StatusSucceeded,
		Message:  "applied and activated",
		Checksum: checksum(cmd.Content),
		Steps:    steps,
	}
}

// rollback restores the pre-apply bytes and reactivates. The returned result keeps the
// ORIGINAL failure as the message: the rollback succeeding does not make the apply a
// success, it only means the box is back where it started.
func rollback(ctx context.Context, p ConfigManager, inst Instance, path string,
	before []byte, existed bool, cause Result, steps []Step) Result {

	var err error
	if existed {
		err = writeFileAtomic(ctx, path, before)
	} else {
		err = removeFile(ctx, path)
	}
	if err != nil {
		steps = append(steps, step("rollback", false, err.Error()))
		return fail(StatusFailed,
			"apply failed AND rollback failed, the config on disk is the rejected one: "+cause.Message,
			steps...)
	}
	// Two separate steps on purpose. "The previous config is back on disk" and "the
	// service picked it up" are different facts, and collapsing them into one line
	// makes a successful restore look like a failed one whenever the reload is what
	// was broken in the first place.
	steps = append(steps, step("rollback", true, "previous config restored on disk"))
	re := p.Activate(ctx, inst, path)
	steps = append(steps, step("rollback-activate", !re.Failed(), re.Message))
	return fail(cause.Status, cause.Message+" (rolled back)", steps...)
}

// writeFileAtomic replaces a file's contents without ever leaving a partial file in
// place: write a sibling temp file, fsync, then rename over the target. The temp file
// is a sibling so the rename stays on one filesystem.
//
// When the agent cannot write the path directly it escalates through the privileged
// helper, which copies the temp file into place instead.
func writeFileAtomic(ctx context.Context, path string, content []byte) error {
	dir := filepath.Dir(path)
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	if writable(path) || dirWritable(dir) {
		tmp, err := os.CreateTemp(dir, ".croncompose-*")
		if err != nil {
			return err
		}
		tmpName := tmp.Name()
		defer os.Remove(tmpName) // no-op once the rename succeeded
		if _, err := tmp.Write(content); err != nil {
			tmp.Close()
			return err
		}
		if err := tmp.Sync(); err != nil {
			tmp.Close()
			return err
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		if err := os.Chmod(tmpName, mode); err != nil {
			return err
		}
		return os.Rename(tmpName, path)
	}

	// Privileged path: stage in the agent's own temp dir, then have a root-owned
	// `install` place it. install(1) writes to a temp inode and renames, so this is
	// still atomic from a reader's point of view.
	tmp, err := os.CreateTemp("", "croncompose-cfg-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	out, ran, err := privRun(ctx, "install", "-m", fmt.Sprintf("%04o", mode), tmpName, path)
	if !ran {
		return errNoPrivilege
	}
	if err != nil {
		return fmt.Errorf("install: %s", trimOutput(out))
	}
	return nil
}

// removeFile deletes a config the apply had created, escalating if needed. Used only
// on the rollback path for a file that did not exist before.
func removeFile(ctx context.Context, path string) error {
	if err := os.Remove(path); err == nil || os.IsNotExist(err) {
		return nil
	}
	out, ran, err := privRun(ctx, "mv", path, path+".croncompose-rolled-back")
	if !ran {
		return errNoPrivilege
	}
	if err != nil {
		return fmt.Errorf("mv: %s", trimOutput(out))
	}
	return nil
}

// usesSystemdHealth reports whether the apply pipeline should ask systemctl if the
// service came back. nginx is a systemd unit (or close enough). pm2 and docker are
// not, and the systemd connector itself manages many units of which most are
// stopped by design — treating "not active" as a failed apply would roll back a
// good drop-in on a disabled unit.
func usesSystemdHealth(kind string) bool {
	switch kind {
	case "pm2", "docker", "systemd":
		return false
	default:
		return true
	}
}

// serviceHealthy re-checks the service a short moment after activation. The pause
// matters: a service that fails on a bad config often exits a beat after the reload
// returns, so checking immediately would report a false success.
func serviceHealthy(ctx context.Context, inst Instance) (bool, string) {
	if !usesSystemdHealth(inst.Kind) {
		return true, "n/a for " + inst.Kind
	}
	unit := inst.Kind
	if inst.Instance != "" {
		unit = inst.Instance
	}
	select {
	case <-time.After(1500 * time.Millisecond):
	case <-ctx.Done():
		return false, "context cancelled during health check"
	}
	st := serviceStatus(ctx, unit)
	if st == "running" || st == "unknown" {
		// `unknown` means we could not tell (no systemctl); do not fail on ignorance.
		return true, st
	}
	return false, "systemd reports " + unit + " is " + st
}

func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".croncompose-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

func step(name string, ok bool, output string) Step {
	return Step{Name: name, OK: ok, Output: trimOutput(output)}
}

func checksum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func shortSum(b []byte) string {
	s := checksum(b)
	return s[:12]
}

func statusForWrite(err error) string {
	if err == errNoPrivilege {
		return StatusUnauthorized
	}
	return StatusFailed
}
