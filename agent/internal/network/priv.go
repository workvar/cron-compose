package network

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// Network ops use the same passwordless-sudo pattern as connectors. Binaries absent
// from this map are never escalated.
var privBinaries = map[string]bool{
	"nmcli":        true,
	"bluetoothctl": true,
	"mmcli":        true,
	"netplan":      true,
	"iw":           true,
	"ip":           true,
}

func needsPriv() bool { return os.Geteuid() != 0 }

func has(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return strings.TrimSpace(buf.String()), err
}

func canSudo(ctx context.Context, bin string) bool {
	if !privBinaries[bin] {
		return false
	}
	if !has("sudo") {
		return false
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return false
	}
	_, err = run(ctx, "sudo", "-n", "-l", path)
	return err == nil
}

func privRun(ctx context.Context, bin string, args ...string) (out string, ran bool, err error) {
	if !needsPriv() {
		out, err = run(ctx, bin, args...)
		return out, true, err
	}
	if !canSudo(ctx, bin) {
		return "", false, nil
	}
	full := append([]string{"-n", bin}, args...)
	out, err = run(ctx, "sudo", full...)
	return out, true, err
}

var errNoPrivilege = errors.New("no privilege: agent is not root and has no passwordless sudo grant")

func mustPriv(ctx context.Context, bin string, args ...string) (string, error) {
	out, ran, err := privRun(ctx, bin, args...)
	if !ran {
		return "", errNoPrivilege
	}
	return out, err
}
