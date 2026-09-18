package deploys

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
	"github.com/croncompose/croncompose/control-plane/internal/audit"
	"github.com/croncompose/croncompose/control-plane/internal/auth"
	"github.com/croncompose/croncompose/control-plane/internal/cryptobox"
	"github.com/croncompose/croncompose/control-plane/internal/githubapp"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

type handler struct {
	log     *slog.Logger
	store   *Store
	conns   *auth.ConnStore
	git     *GitAPI
	gateway *agentgw.Gateway
	audit   audit.Writer
	public  string
	box     *cryptobox.Box
	// app is the optional GitHub App used to post commit statuses as CronCompose
	// rather than as the user who imported the repo. nil means none is configured.
	app *githubapp.App
}

func jsonError(c fiber.Ctx, status int, code string, err error) error {
	msg := code
	if err != nil {
		msg = err.Error()
	}
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": msg}})
}

func (h *handler) listConnections(c fiber.Ctx) error {
	items, err := h.conns.ListGit(c.Context(), auth.CurrentUserID(c))
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *handler) deleteConnection(c fiber.Ctx) error {
	provider := c.Params("provider")
	if err := h.conns.DeleteGit(c.Context(), auth.CurrentUserID(c), provider); err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "delete_failed", err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *handler) listRepos(c fiber.Ctx) error {
	provider := c.Query("provider", "github")
	token, err := h.conns.Token(c.Context(), auth.CurrentUserID(c), provider)
	if errors.Is(err, auth.ErrNotFound) {
		return jsonError(c, fiber.StatusConflict, "not_connected", errors.New("connect "+provider+" in Settings first"))
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "token_failed", err)
	}
	items, err := h.git.ListRepos(c.Context(), provider, token)
	if err != nil {
		return jsonError(c, fiber.StatusBadGateway, "git_api", err)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *handler) inspect(c fiber.Ctx) error {
	provider := c.Query("provider", "github")
	repo := c.Query("repo")
	branch := c.Query("branch")
	if repo == "" {
		return jsonError(c, fiber.StatusBadRequest, "missing_repo", errors.New("repo is required"))
	}
	token, err := h.conns.Token(c.Context(), auth.CurrentUserID(c), provider)
	if errors.Is(err, auth.ErrNotFound) {
		return jsonError(c, fiber.StatusConflict, "not_connected", errors.New("connect "+provider+" in Settings first"))
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "token_failed", err)
	}
	files, meta, err := h.git.FetchFiles(c.Context(), provider, token, repo, branch)
	if err != nil {
		return jsonError(c, fiber.StatusBadGateway, "git_api", err)
	}
	det := Detect(files)
	settings, _ := h.store.GetSettings(c.Context())
	hint := "none"
	if det.HasPM2Ecosystem {
		hint = "pm2"
	}
	return c.JSON(Inspect{
		Detection:     det,
		CloneURL:      meta.CloneURL,
		DefaultBranch: meta.DefaultBranch,
		ClonePath:     ClonePath(settings.LanguagePaths, det.Language, repo),
		ProcessHint:   hint,
	})
}

func (h *handler) getSettings(c fiber.Ctx) error {
	s, err := h.store.GetSettings(c.Context())
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	return c.JSON(s)
}

func (h *handler) putSettings(c fiber.Ctx) error {
	var in struct {
		LanguagePaths map[string]string `json:"language_paths"`
	}
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	s, err := h.store.PutSettings(c.Context(), in.LanguagePaths)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "save_failed", err)
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "deploy.settings", "settings", "default", nil)
	return c.JSON(s)
}

func (h *handler) list(c fiber.Ctx) error {
	items, err := h.store.List(c.Context())
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	for i := range items {
		items[i] = RedactProject(items[i])
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *handler) get(c fiber.Ctx) error {
	p, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	secret, _ := h.store.WebhookSecret(c.Context(), p.ID)
	base := strings.TrimRight(h.public, "/")
	return c.JSON(fiber.Map{
		"project":        RedactProject(p),
		"webhook_secret": secret,
		"webhook_url":    base + "/api/deploys/webhooks/" + p.Provider,
	})
}

func (h *handler) create(c fiber.Ctx) error {
	var in CreateInput
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	if in.Provider == "" || in.RepoFullName == "" || in.ServerID == "" {
		return jsonError(c, fiber.StatusBadRequest, "missing_fields", errors.New("provider, repo_full_name and server_id are required"))
	}
	settings, _ := h.store.GetSettings(c.Context())
	if in.ClonePath == "" {
		in.ClonePath = ClonePath(settings.LanguagePaths, in.Language, in.RepoFullName)
	}
	if in.CloneURL == "" {
		in.CloneURL = httpsCloneURL(in.Provider, in.RepoFullName)
	}
	apps, err := normalizeCreateApps(h.box, in)
	if err != nil {
		return jsonError(c, fiber.StatusBadRequest, "env_invalid", err)
	}
	in.Apps = apps
	p, err := h.store.Insert(c.Context(), in, auth.CurrentUserID(c))
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "insert_failed", err)
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "deploy.create", "deploy", p.ID, map[string]any{"repo": p.RepoFullName})
	warnings := h.provisionRemote(c.Context(), p)
	run, err := h.startRun(c.Context(), p, "manual", p.DefaultBranch, "")
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "run_failed", err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"project": RedactProject(p), "run": run, "warnings": warnings})
}

func (h *handler) patch(c fiber.Ctx) error {
	var in UpdateInput
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	before, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	if in.Apps != nil {
		merged, err := MergeAppsEnv(h.box, before.Apps, *in.Apps)
		if err != nil {
			return jsonError(c, fiber.StatusBadRequest, "env_invalid", err)
		}
		in.Apps = &merged
	}
	p, err := h.store.Update(c.Context(), c.Params("id"), in)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "update_failed", err)
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "deploy.update", "deploy", p.ID, nil)
	out := fiber.Map{"project": RedactProject(p)}
	pmChanged := in.ProcessManager != nil && p.ProcessManager != before.ProcessManager
	if pmChanged && p.ProcessManager != "" && p.ProcessManager != "none" {
		run, err := h.startRun(c.Context(), p, "manual", p.DefaultBranch, "")
		if err != nil {
			return jsonError(c, fiber.StatusInternalServerError, "run_failed", err)
		}
		out["run"] = run
	}
	return c.JSON(out)
}

func (h *handler) remove(c fiber.Ctx) error {
	id := c.Params("id")
	if err := h.store.Delete(c.Context(), id); errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	} else if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "delete_failed", err)
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "deploy.delete", "deploy", id, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *handler) provisionRemote(ctx context.Context, p Project) []string {
	var warnings []string
	token := ""
	if p.CreatedBy != nil {
		token, _ = h.conns.Token(ctx, *p.CreatedBy, p.Provider)
	}
	if token == "" {
		return []string{"git grant missing; webhook and repo files were not written"}
	}
	secret, _ := h.store.WebhookSecret(ctx, p.ID)
	hookURL := strings.TrimRight(h.public, "/") + "/api/deploys/webhooks/" + p.Provider
	if err := h.git.EnsureWebhook(ctx, p.Provider, token, p.RepoFullName, p.RepoID, hookURL, secret); err != nil {
		warnings = append(warnings, "webhook: "+err.Error())
	}
	if p.WriteSpec {
		if err := h.git.EnsureRepoFiles(ctx, p.Provider, token, p.RepoFullName, p.RepoID, p.DefaultBranch, specFiles(p, h.public)); err != nil {
			warnings = append(warnings, "repo files: "+err.Error())
		}
	}
	return warnings
}

func (h *handler) workflow(c fiber.Ctx) error {
	p, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	spec, _ := MarshalSpec(Spec{
		Name: p.Name, Provider: p.Provider, Repo: p.RepoFullName, Branch: p.DefaultBranch,
		Language: p.Language, Install: p.InstallScript, Root: p.RootDirectory,
		Port: p.Port, ProcessManager: p.ProcessManager, ClonePath: p.ClonePath,
		Apps: p.Apps, Env: p.Env,
	})
	wf := GitHubActionsWorkflow(h.public, p.ID)
	if p.Provider == "gitlab" {
		wf = GitLabCI(h.public, p.ID)
	}
	return c.JSON(fiber.Map{
		"croncompose_yml": string(spec),
		"workflow_yml":    wf,
	})
}

func (h *handler) createRun(c fiber.Ctx) error {
	p, err := h.store.Get(c.Context(), c.Params("id"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	trigger := "manual"
	if auth.HasMinRole(c, "operator") {
		trigger = "manual"
	} else {
		hash, err := h.store.TokenHash(c.Context(), p.ID)
		if err != nil {
			return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
		}
		if !MatchToken(hash, bearerToken(c)) {
			if auth.CurrentUserID(c) != "" {
				return jsonError(c, fiber.StatusForbidden, "forbidden", errors.New("insufficient role"))
			}
			return jsonError(c, fiber.StatusUnauthorized, "unauthenticated", errors.New("deploy token required"))
		}
		trigger = "api"
	}
	var body struct {
		Trigger string `json:"trigger"`
		Branch  string `json:"branch"`
		Commit  string `json:"commit"`
	}
	_ = c.Bind().Body(&body)
	// "rollback" is reserved for runs this control plane starts itself: a caller able
	// to set it would silently opt the project out of auto-rollback, since a run that
	// is already a rollback never triggers another one.
	if body.Trigger != "" && body.Trigger != triggerRollback {
		trigger = body.Trigger
	}
	branch := body.Branch
	if branch == "" {
		branch = p.DefaultBranch
	}
	// De-duplication applies to automated triggers only. An operator clicking Redeploy
	// has decided they want this run, even if one is already going.
	if trigger != "manual" {
		if reason := h.duplicateTrigger(c.Context(), p, body.Commit); reason != "" {
			h.log.Info("deploy: skipping duplicate trigger", "project_id", p.ID, "reason", reason)
			return c.JSON(fiber.Map{"skipped": reason})
		}
	}
	run, err := h.startRun(c.Context(), p, trigger, branch, "")
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "run_failed", err)
	}
	return c.Status(fiber.StatusAccepted).JSON(run)
}

func (h *handler) listRuns(c fiber.Ctx) error {
	items, err := h.store.ListRuns(c.Context(), c.Params("id"), 50)
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "list_failed", err)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *handler) getRun(c fiber.Ctx) error {
	r, err := h.store.GetRun(c.Context(), c.Params("runId"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	return c.JSON(r)
}

func (h *handler) stdin(c fiber.Ctx) error {
	run, err := h.store.GetRun(c.Context(), c.Params("runId"))
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	var in struct {
		Data string `json:"data"`
	}
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	if err := h.gateway.SendDeploy(run.ServerID, &agentv1.DeployCommand{
		RunId: run.ID, Op: "stdin", Stdin: []byte(in.Data),
	}); err != nil {
		return jsonError(c, fiber.StatusServiceUnavailable, "agent_offline", err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *handler) stream(c fiber.Ctx) error {
	runID := c.Params("runId")
	run, err := h.store.GetRun(c.Context(), runID)
	if errors.Is(err, ErrNotFound) {
		return jsonError(c, fiber.StatusNotFound, "not_found", err)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "get_failed", err)
	}
	sub := h.gateway.Broker().Subscribe(runID)
	snapshot, _ := h.store.Logs(c.Context(), runID)
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")
	done := run.Status != "pending" && run.Status != "running"
	c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer h.gateway.Broker().Unsubscribe(runID, sub)
		for _, l := range snapshot {
			writeEvent(w, "log", fmt.Sprintf(`{"stream":%q,"seq":%d,"chunk":%q}`, l.Stream, l.Seq, l.Chunk))
		}
		if done {
			writeEvent(w, "done", fmt.Sprintf(`{"status":%q}`, run.Status))
			return
		}
		for ev := range sub {
			if ev.Chunk != nil {
				writeEvent(w, "log", fmt.Sprintf(`{"stream":%q,"seq":%d,"chunk":%q}`,
					ev.Chunk.GetStream(), ev.Chunk.GetSeq(), string(ev.Chunk.GetData())))
			}
			if ev.Finished != nil {
				writeEvent(w, "done", fmt.Sprintf(`{"status":%q,"exit_code":%d}`,
					ev.Finished.GetStatus(), ev.Finished.GetExitCode()))
				return
			}
		}
	})
	return nil
}

func writeEvent(w *bufio.Writer, event, data string) {
	_, _ = w.WriteString("event: " + event + "\ndata: " + data + "\n\n")
	_ = w.Flush()
}

func (h *handler) githubWebhook(c fiber.Ctx) error {
	body := c.Body()
	var payload struct {
		Ref        string `json:"ref"`
		After      string `json:"after"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_json", err)
	}
	branch := BranchFromRef(payload.Ref)
	if branch == "" {
		return c.SendStatus(fiber.StatusNoContent)
	}
	p, secret, err := h.store.GetByRepo(c.Context(), "github", payload.Repository.FullName)
	if errors.Is(err, ErrNotFound) {
		return c.SendStatus(fiber.StatusNoContent)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "lookup_failed", err)
	}
	if !ValidGitHubSignature(secret, c.Get("X-Hub-Signature-256"), body) {
		return jsonError(c, fiber.StatusUnauthorized, "bad_signature", errors.New("bad signature"))
	}
	if branch != p.DefaultBranch {
		return c.SendStatus(fiber.StatusNoContent)
	}
	return h.triggerFromWebhook(c, p, branch, c.Get("X-GitHub-Delivery"), payload.After)
}

func (h *handler) gitlabWebhook(c fiber.Ctx) error {
	body := c.Body()
	var payload struct {
		Ref         string `json:"ref"`
		CheckoutSHA string `json:"checkout_sha"`
		Project     struct {
			Path string `json:"path_with_namespace"`
		} `json:"project"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_json", err)
	}
	branch := BranchFromRef(payload.Ref)
	if branch == "" {
		return c.SendStatus(fiber.StatusNoContent)
	}
	p, secret, err := h.store.GetByRepo(c.Context(), "gitlab", payload.Project.Path)
	if errors.Is(err, ErrNotFound) {
		return c.SendStatus(fiber.StatusNoContent)
	}
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "lookup_failed", err)
	}
	if !ValidGitLabToken(secret, c.Get("X-Gitlab-Token")) {
		return jsonError(c, fiber.StatusUnauthorized, "bad_token", errors.New("bad token"))
	}
	if branch != p.DefaultBranch {
		return c.SendStatus(fiber.StatusNoContent)
	}
	return h.triggerFromWebhook(c, p, branch, c.Get("X-Gitlab-Event-UUID"), payload.CheckoutSHA)
}

// startRun starts a deploy. pinSHA is empty for every normal deploy (branch tip); an
// automatic rollback (see DeployRunFinished) sets it to the commit to return to, and
// the agent prefers a release already on disk carrying that commit over refetching
// it, because the commit may be gone from the remote by then.
func (h *handler) startRun(ctx context.Context, p Project, trigger, branch, pinSHA string) (Run, error) {
	// A rollback already knows its commit; a push or CI trigger reported one, and it
	// reached us through duplicateTrigger. Recording it now rather than waiting for the
	// agent lets the commit status go pending immediately and makes the de-duplication
	// window cover a run that has not checked out yet.
	run, err := h.store.InsertRun(ctx, p.ID, p.ServerID, trigger, branch, pinSHA)
	if err != nil {
		return Run{}, err
	}
	h.reportStatusStart(ctx, p, run)
	token := ""
	if p.CreatedBy != nil {
		token, _ = h.conns.Token(ctx, *p.CreatedBy, p.Provider)
	}
	cmd := &agentv1.DeployCommand{
		RunId: run.ID, Op: "start",
		CloneUrl: p.CloneURL, CloneToken: token,
		DestPath: p.ClonePath, Branch: branch,
		InstallScript: p.InstallScript, RootDirectory: p.RootDirectory,
		Env: p.Env, Port: int32(p.Port), ProcessManager: p.ProcessManager,
		RollbackSha:    pinSHA,
		Health:         healthCheckFor(p),
		TimeoutSeconds: int32(p.DeployTimeoutSeconds),
	}
	for _, a := range p.Apps {
		lang, pm := a.Language, a.ProcessManager
		if lang == "" {
			lang = p.Language
		}
		if pm == "" {
			pm = p.ProcessManager
		}
		appEnv, err := ResolveAppEnv(h.box, a)
		if err != nil {
			return Run{}, err
		}
		// Legacy project-level env fills gaps; per-app wins.
		merged := map[string]string{}
		for k, v := range p.Env {
			merged[k] = v
		}
		for k, v := range appEnv {
			merged[k] = v
		}
		cmd.Apps = append(cmd.Apps, &agentv1.DeployApp{
			Name: a.Name, RootDirectory: a.Root, InstallScript: a.Install,
			Language: lang, Port: int32(a.Port), ProcessManager: pm, Env: merged,
		})
	}
	if len(cmd.Apps) == 0 {
		cmd.Apps = []*agentv1.DeployApp{{
			Name: p.Name, RootDirectory: p.RootDirectory, InstallScript: p.InstallScript,
			Language: p.Language, Port: int32(p.Port), ProcessManager: p.ProcessManager, Env: p.Env,
		}}
	}
	if err := h.gateway.SendDeploy(p.ServerID, cmd); err != nil {
		_ = h.store.MarkRun(ctx, run.ID, "agent_offline", 0, err.Error())
		run.Status = "agent_offline"
		run.Error = err.Error()
		return run, nil
	}
	_ = h.store.MarkRun(ctx, run.ID, "running", 0, "")
	run.Status = "running"
	return run, nil
}

func bearerToken(c fiber.Ctx) string {
	h := c.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}
