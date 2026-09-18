package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

const defaultPrivctlPath = "/usr/libexec/croncompose/agent-privctl"

// handleAgentRootCommand runs the privileged helper to elevate (enabled=true) or
// demote (enabled=false). Disable always attempts demotion. Failures are logged and
// pushed on the ephemeral direct-send path so the control plane can surface them.
func (r *Runtime) handleAgentRootCommand(enabled bool) {
	action := "demote"
	if enabled {
		action = "elevate"
	}
	if err := r.runPrivctl(action); err != nil {
		r.log.Error("agent root command failed", "action", action, "err", err)
		r.sendDirect(&agentv1.AgentMessage{
			Body: &agentv1.AgentMessage_UpdateProgress{UpdateProgress: &agentv1.UpdateProgress{
				Phase:  "failed",
				Detail: err.Error(),
			}},
		})
	} else {
		r.log.Info("agent root command sent", "action", action)
	}
}

func (r *Runtime) runPrivctl(action string) error {
	helper := strings.TrimSpace(os.Getenv("CC_AGENT_PRIVCTL"))
	if helper == "" {
		helper = defaultPrivctlPath
	}
	sudo := strings.TrimSpace(os.Getenv("CC_AGENT_PRIVCTL_SUDO"))
	if sudo == "" {
		sudo = "sudo"
	}
	cmd := exec.Command(sudo, "-n", helper, action)
	cmd.Env = os.Environ()
	if r.cfg.DataDir != "" {
		cmd.Env = append(cmd.Env, "DATA_DIR="+r.cfg.DataDir)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s: %s; grant: ALL=(root) NOPASSWD: %s elevate, %s demote",
			action, msg, defaultPrivctlPath, defaultPrivctlPath)
	}
	return nil
}
