package deploys

import (
	"strings"

	"github.com/croncompose/croncompose/control-plane/internal/cryptobox"
)

// normalizeCreateApps seals per-app env and, when apps is empty but project env
// was provided as a flat map, attaches that map to a single default app.
func normalizeCreateApps(box *cryptobox.Box, in CreateInput) ([]SpecApp, error) {
	apps := in.Apps
	if len(apps) == 0 {
		root := in.RootDirectory
		if root == "" {
			root = "."
		}
		name := in.Name
		if name == "" {
			name = in.RepoFullName
		}
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		var env []EnvVar
		for k, v := range in.Env {
			env = append(env, EnvVar{Key: k, Value: v})
		}
		apps = []SpecApp{{
			Name: name, Root: root, Language: in.Language, Install: in.InstallScript,
			Port: in.Port, ProcessManager: in.ProcessManager, Env: env,
		}}
	}
	return SealAppsEnv(box, apps)
}
