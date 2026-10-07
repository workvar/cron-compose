package deploys

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/auth"
)

// ImportProcessInput is POST /servers/:id/deploys/import-process.
type ImportProcessInput struct {
	Kind           string            `json:"kind"` // pm2|systemd|docker
	Ref            string            `json:"ref"`
	Name           string            `json:"name"`
	Command        string            `json:"command,omitempty"`
	Cwd            string            `json:"cwd,omitempty"`
	Provider       string            `json:"provider"`
	RepoFullName   string            `json:"repo_full_name"`
	CloneURL       string            `json:"clone_url,omitempty"`
	DefaultBranch  string            `json:"default_branch,omitempty"`
	Language       string            `json:"language,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	ProcessManager string            `json:"process_manager,omitempty"`
}

func (h *handler) importProcess(c fiber.Ctx) error {
	serverID := c.Params("id")
	var in ImportProcessInput
	if err := c.Bind().Body(&in); err != nil {
		return jsonError(c, fiber.StatusBadRequest, "bad_request", err)
	}
	in.Kind = strings.TrimSpace(strings.ToLower(in.Kind))
	in.Name = strings.TrimSpace(in.Name)
	in.Provider = strings.TrimSpace(strings.ToLower(in.Provider))
	in.RepoFullName = strings.TrimSpace(in.RepoFullName)
	if in.Kind == "" || in.Ref == "" || in.Name == "" {
		return jsonError(c, fiber.StatusBadRequest, "missing_fields",
			errors.New("kind, ref, and name are required"))
	}
	if in.Kind != "pm2" && in.Kind != "systemd" && in.Kind != "docker" {
		return jsonError(c, fiber.StatusBadRequest, "bad_kind",
			errors.New("kind must be pm2, systemd, or docker"))
	}
	if in.Provider == "" || in.RepoFullName == "" {
		return jsonError(c, fiber.StatusBadRequest, "missing_fields",
			errors.New("provider and repo_full_name are required"))
	}
	pm := in.ProcessManager
	if pm == "" {
		pm = in.Kind
		if pm == "docker" {
			pm = "docker"
		}
	}
	falseSpec := false
	create := CreateInput{
		Name:           in.Name,
		Provider:       in.Provider,
		RepoFullName:   in.RepoFullName,
		CloneURL:       in.CloneURL,
		DefaultBranch:  in.DefaultBranch,
		ServerID:       serverID,
		Language:       in.Language,
		InstallScript:  in.Command,
		RootDirectory:  ".",
		ClonePath:      in.Cwd,
		ProcessManager: pm,
		Env:            in.Env,
		WriteSpec:      &falseSpec,
		Apps: []SpecApp{{
			Name:           in.Name,
			Root:           ".",
			Language:       in.Language,
			Install:        in.Command,
			ProcessManager: pm,
			Env:            envMapToVars(in.Env),
		}},
	}
	settings, _ := h.store.GetSettings(c.Context())
	if create.ClonePath == "" {
		create.ClonePath = ClonePath(settings.LanguagePaths, create.Language, create.RepoFullName)
	}
	if create.CloneURL == "" {
		create.CloneURL = httpsCloneURL(create.Provider, create.RepoFullName)
	}
	apps, err := normalizeCreateApps(h.box, create)
	if err != nil {
		return jsonError(c, fiber.StatusBadRequest, "env_invalid", err)
	}
	create.Apps = apps

	p, err := h.store.Insert(c.Context(), create, auth.CurrentUserID(c))
	if err != nil {
		return jsonError(c, fiber.StatusInternalServerError, "insert_failed", err)
	}
	h.audit.Write(c.Context(), auth.CurrentUserID(c), "deploy.import_process", "deploy", p.ID, map[string]any{
		"kind": in.Kind, "ref": in.Ref, "server_id": serverID, "repo": p.RepoFullName,
	})
	// Intentionally no startRun and no webhook provisioning: the process is already
	// running; import only registers it as a Deploy Project.
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"project": RedactProject(p)})
}

func envMapToVars(m map[string]string) []EnvVar {
	if len(m) == 0 {
		return nil
	}
	out := make([]EnvVar, 0, len(m))
	for k, v := range m {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		// Imported process env is treated as sensitive by default.
		out = append(out, EnvVar{Key: k, Value: v, Sensitive: true})
	}
	return out
}
