//go:build !unix

package sourceupdate

import "os/exec"

func detach(cmd *exec.Cmd) {}
