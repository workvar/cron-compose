package deploy

import (
	"context"
	"sort"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

func stepKey(app, phase string) string {
	return app + "\x00" + phase
}

func stepCmd(cmd *agentv1.DeployCommand) (runID, app, phase string) {
	if cmd == nil {
		return "", "", ""
	}
	runID = cmd.GetRunId()
	if apps := cmd.GetApps(); len(apps) > 0 {
		app = apps[0].GetName()
	}
	if waves := cmd.GetFlow(); len(waves) > 0 && len(waves[0].GetTasks()) > 0 {
		t := waves[0].GetTasks()[0]
		if app == "" {
			app = t.GetApp()
		}
		phase = t.GetPhase()
	}
	return runID, app, phase
}

// skipStep marks one app's phase to be skipped. If that phase is running, its
// command is canceled and the flow continues with this app's next step.
func (m *Manager) skipStep(cmd *agentv1.DeployCommand) {
	runID, app, phase := stepCmd(cmd)
	if runID == "" || app == "" || phase == "" {
		return
	}
	m.mu.Lock()
	if m.stepSkip[runID] == nil {
		m.stepSkip[runID] = map[string]bool{}
	}
	m.stepSkip[runID][stepKey(app, phase)] = true
	running := m.taskPhase[runID][app]
	cancel := m.appCancel[runID][app]
	if running == phase {
		if m.appHalt[runID] == nil {
			m.appHalt[runID] = map[string]string{}
		}
		m.appHalt[runID][app] = "skipped"
	}
	m.mu.Unlock()
	if running == phase && cancel != nil {
		cancel()
	}
}

// stopStep cancels the app's in-flight step and skips the rest of that app.
// Other apps in the same deploy keep running.
func (m *Manager) stopStep(cmd *agentv1.DeployCommand) {
	runID, app, _ := stepCmd(cmd)
	if runID == "" || app == "" {
		return
	}
	m.mu.Lock()
	if m.appHalt[runID] == nil {
		m.appHalt[runID] = map[string]string{}
	}
	m.appHalt[runID][app] = "stopped"
	cancel := m.appCancel[runID][app]
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *Manager) takeSkip(runID, app, phase string) bool {
	if app == "" || phase == "" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := stepKey(app, phase)
	if !m.stepSkip[runID][key] {
		return false
	}
	delete(m.stepSkip[runID], key)
	return true
}

func (m *Manager) beginTask(runID, app, phase string, cancel context.CancelFunc) {
	if app == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.taskPhase[runID] == nil {
		m.taskPhase[runID] = map[string]string{}
	}
	if m.appCancel[runID] == nil {
		m.appCancel[runID] = map[string]context.CancelFunc{}
	}
	m.taskPhase[runID][app] = phase
	m.appCancel[runID][app] = cancel
}

func (m *Manager) endTask(runID, app string) {
	if app == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.taskPhase[runID], app)
	delete(m.appCancel[runID], app)
}

// haltOf returns a one-shot "skipped" (cleared) or a sticky "stopped".
func (m *Manager) haltOf(runID, app string) string {
	if app == "" {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	why := m.appHalt[runID][app]
	if why == "skipped" {
		delete(m.appHalt[runID], app)
	}
	return why
}

func (m *Manager) stickyHalt(runID, app string) string {
	if app == "" {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.appHalt[runID][app] == "stopped" {
		return "stopped"
	}
	return ""
}

func (m *Manager) stoppedApps(runID string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for app, why := range m.appHalt[runID] {
		if why == "stopped" {
			out = append(out, app)
		}
	}
	sort.Strings(out)
	return out
}

// runGuarded runs fn for one app phase. Skip and stop apply to that app only.
func (m *Manager) runGuarded(ctx context.Context, runID, token, app, flowPhase, work string, fn func(context.Context) error) error {
	logPhase, skippedMsg := stepStatusLine(flowPhase, "skipped", app, work)
	if m.takeSkip(runID, app, flowPhase) {
		m.phaseLine(runID, token, logPhase, skippedMsg)
		return nil
	}
	if why := m.stickyHalt(runID, app); why != "" {
		ph, msg := stepStatusLine(flowPhase, why, app, work)
		m.phaseLine(runID, token, ph, msg)
		return nil
	}
	appCtx, cancel := context.WithCancel(ctx)
	m.beginTask(runID, app, flowPhase, cancel)
	err := fn(appCtx)
	cancel()
	m.endTask(runID, app)
	if err != nil {
		if why := m.haltOf(runID, app); why != "" {
			ph, msg := stepStatusLine(flowPhase, why, app, work)
			m.phaseLine(runID, token, ph, msg)
			return nil
		}
		return err
	}
	return nil
}

func stepStatusLine(flowPhase, why, app, work string) (logPhase, msg string) {
	switch flowPhase {
	case "build":
		logPhase = phaseInstall
	case "cleanup":
		logPhase = phaseInstall
		if work != "" {
			return logPhase, "cleanup: " + why + " (in " + work + ")"
		}
		return logPhase, "cleanup: " + why
	case "health":
		if app == "" {
			app = "app"
		}
		return phaseHealth, why + ": " + app
	default:
		logPhase = flowPhase
		if logPhase == "" {
			logPhase = phaseInstall
		}
	}
	if work != "" {
		return logPhase, why + " (in " + work + ")"
	}
	return logPhase, why
}
