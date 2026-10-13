package deploy

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/croncompose/croncompose/agent/internal/osuser"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

var preReleasePhases = map[string]bool{
	"install": true, "build": true, "cleanup": true,
}
var postReleasePhases = map[string]bool{
	"pre_start": true, "start": true, "health": true,
}

// runDeployFlow executes an explicit flow: waves of {app, phase} tasks.
// Release activates once, between the last pre-release wave and the first
// post-release wave. Empty flow falls back to the caller.
func (m *Manager) runDeployFlow(
	ctx context.Context,
	cmd *agentv1.DeployCommand,
	layout Layout,
	cred *osuser.Credential,
	tmpDir, release string,
) error {
	runID := cmd.GetRunId()
	token := cmd.GetCloneToken()
	continueOnFail := strings.EqualFold(strings.TrimSpace(cmd.GetOnWaveFailure()), "continue")
	apps := appsOf(cmd, release)
	byName := map[string]*agentv1.DeployApp{}
	for _, a := range apps {
		byName[a.GetName()] = a
	}

	waves := cmd.GetFlow()
	released := false
	var waveErrs []error

	for i, wave := range waves {
		tasks := wave.GetTasks()
		if len(tasks) == 0 {
			continue
		}
		needsPost := false
		needsPre := false
		for _, t := range tasks {
			ph := strings.ToLower(strings.TrimSpace(t.GetPhase()))
			if postReleasePhases[ph] {
				needsPost = true
			}
			if preReleasePhases[ph] {
				needsPre = true
			}
		}
		if needsPost && !released {
			if needsPre {
				return phaseError{phaseInstall, fmt.Errorf("flow wave %d mixes pre-release and post-release tasks", i+1)}
			}
			if err := layout.Swap(release); err != nil {
				return phaseError{phaseRelease, fmt.Errorf("activate release: %w", err)}
			}
			_ = chownTree(layout.Base, cred)
			m.phaseLine(runID, token, phaseRelease, "current -> "+filepath.Base(release))
			released = true
			apps = appsOf(cmd, layout.Current)
			byName = map[string]*agentv1.DeployApp{}
			for _, a := range apps {
				byName[a.GetName()] = a
			}
		}

		workRoot := release
		if released {
			workRoot = layout.Current
		}

		if len(waves) > 1 {
			labels := make([]string, 0, len(tasks))
			for _, t := range tasks {
				labels = append(labels, t.GetApp()+"·"+t.GetPhase())
			}
			m.phaseLine(runID, token, phaseInstall, fmt.Sprintf("flow wave %d/%d: %s", i+1, len(waves), strings.Join(labels, ", ")))
		}

		if err := m.runFlowWave(ctx, cmd, layout, workRoot, released, cred, tmpDir, byName, tasks); err != nil {
			if !continueOnFail {
				return err
			}
			m.phaseLine(runID, token, phaseInstall, "wave failed; continuing: "+err.Error())
			waveErrs = append(waveErrs, err)
			continue
		}
	}

	if !released {
		if err := layout.Swap(release); err != nil {
			return phaseError{phaseRelease, fmt.Errorf("activate release: %w", err)}
		}
		_ = chownTree(layout.Base, cred)
		m.phaseLine(runID, token, phaseRelease, "current -> "+filepath.Base(release))
	}

	if err := m.persistProcessManagers(ctx, cmd, layout, cred, tmpDir); err != nil {
		return phaseError{phaseStart, err}
	}
	if err := joinErrors(waveErrs); err != nil {
		return phaseError{phaseStart, err}
	}
	return nil
}

func (m *Manager) runFlowWave(
	ctx context.Context,
	cmd *agentv1.DeployCommand,
	layout Layout,
	workRoot string,
	released bool,
	cred *osuser.Credential,
	tmpDir string,
	byName map[string]*agentv1.DeployApp,
	tasks []*agentv1.DeployFlowTask,
) error {
	runID := cmd.GetRunId()
	token := cmd.GetCloneToken()

	var scriptJobs []appScript
	var startApps []*agentv1.DeployApp
	var healthApps []*agentv1.DeployApp

	for _, t := range tasks {
		app := byName[t.GetApp()]
		if app == nil {
			return phaseError{phaseInstall, fmt.Errorf("flow references unknown app %q", t.GetApp())}
		}
		phase := strings.ToLower(strings.TrimSpace(t.GetPhase()))
		work := appWorkDir(workRoot, cmd, app)
		switch phase {
		case "install", "build":
			script := app.GetInstallScript()
			if script == "" {
				script = cmd.GetInstallScript()
			}
			inst, build := splitInstallBuild(script, app.GetLanguage())
			body := inst
			if phase == "build" {
				body = build
			}
			if body == "" {
				continue
			}
			scriptJobs = append(scriptJobs, appScript{
				label:  body,
				work:   work,
				script: body,
				env:    appEnv(cmd, app),
				phase:  body,
				done:   "done",
			})
		case "cleanup":
			script := cleanupScriptFor(app.GetCleanupScript(), app.GetLanguage())
			if script == "" {
				continue
			}
			scriptJobs = append(scriptJobs, appScript{
				label:  "cleanup",
				work:   work,
				script: script,
				env:    appEnv(cmd, app),
				phase:  "cleanup: " + script,
			})
		case "pre_start":
			script := strings.TrimSpace(app.GetPreStartScript())
			if script == "" {
				continue
			}
			scriptJobs = append(scriptJobs, appScript{
				label:  "pre-start",
				work:   work,
				script: script,
				env:    appEnv(cmd, app),
				phase:  script,
				done:   "done",
			})
		case "start":
			startApps = append(startApps, app)
		case "health":
			healthApps = append(healthApps, app)
		default:
			return phaseError{phaseInstall, fmt.Errorf("unknown flow phase %q", t.GetPhase())}
		}
	}

	logPhase := phaseInstall
	if len(scriptJobs) > 0 {
		// Prefer pre_start phase label when every script job is pre_start.
		allPre := true
		for _, j := range scriptJobs {
			if j.label != "pre-start" {
				allPre = false
				break
			}
		}
		if allPre {
			logPhase = phasePreStart
		}
		if err := m.runAppScripts(ctx, runID, token, cred, tmpDir, logPhase, scriptJobs); err != nil {
			if logPhase == phasePreStart {
				return phaseError{phasePreStart, err}
			}
			return phaseError{phaseInstall, err}
		}
	}
	if len(startApps) > 0 {
		if !released {
			return phaseError{phaseStart, fmt.Errorf("start tasks require an activated release")}
		}
		if err := m.startApps(ctx, cmd, layout, cred, tmpDir, startApps); err != nil {
			return phaseError{phaseStart, err}
		}
	}
	if len(healthApps) > 0 {
		if err := m.checkHealth(ctx, cmd, healthApps); err != nil {
			return phaseError{phaseHealth, err}
		}
	}
	return nil
}
