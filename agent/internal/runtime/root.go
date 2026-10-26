package runtime

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/croncompose/croncompose/agent/internal/platform"
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
	if platform.IsWindows() {
		return windowsRootCommand(action)
	}
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

// windowsRootCommand answers elevate/demote on Windows, where there is no sudo or
// privctl helper. The service runs as LocalSystem (the root equivalent), so elevate
// is already true and demote would mean changing the service's logon account.
func windowsRootCommand(action string) error {
	if action == "elevate" && platform.IsElevated() {
		return nil
	}
	if action == "elevate" {
		return errors.New("elevate: run the agent as the Windows service (LocalSystem); " +
			"reinstall with install-agent.ps1 from an Administrator PowerShell")
	}
	return errors.New("demote: the Windows agent service runs as LocalSystem; to use a " +
		"less privileged account, change the croncompose-agent service's Log On account in services.msc")
}
