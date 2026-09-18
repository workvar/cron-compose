package deploys

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/croncompose/croncompose/control-plane/internal/cryptobox"
)

// EnvVar is one environment variable on a deploy app.
// Sensitive values are stored as ValueEnc (hex of cryptobox.Seal) and never
// returned to the browser; non-sensitive values stay in Value.
type EnvVar struct {
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
	ValueEnc  string `json:"value_enc,omitempty"`
	Sensitive bool   `json:"sensitive"`
	HasValue  bool   `json:"has_value,omitempty"`
}

// ParseDotEnv parses a Vercel-style .env blob into a map (last key wins).
func ParseDotEnv(raw string) map[string]string {
	out := map[string]string{}
	for _, v := range ParseDotEnvOrdered(raw) {
		out[v.Key] = v.Value
	}
	return out
}

// ParseDotEnvOrdered preserves first-seen order (for UI paste).
func ParseDotEnvOrdered(raw string) []EnvVar {
	var out []EnvVar
	seen := map[string]int{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		key = strings.Trim(key, `"'`)
		if key == "" || strings.ContainsAny(key, " \t") {
			continue
		}
		val := strings.TrimSpace(line[i+1:])
		val = unquoteEnv(val)
		if idx, ok := seen[key]; ok {
			out[idx].Value = val
			continue
		}
		seen[key] = len(out)
		out = append(out, EnvVar{Key: key, Value: val})
	}
	return out
}

func unquoteEnv(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

// SealAppsEnv encrypts sensitive values and clears their plaintext.
func SealAppsEnv(box *cryptobox.Box, apps []SpecApp) ([]SpecApp, error) {
	out := make([]SpecApp, len(apps))
	for i, a := range apps {
		out[i] = a
		env, err := sealEnv(box, a.Env)
		if err != nil {
			return nil, err
		}
		out[i].Env = env
	}
	return out, nil
}

func sealEnv(box *cryptobox.Box, vars []EnvVar) ([]EnvVar, error) {
	if vars == nil {
		return nil, nil
	}
	out := make([]EnvVar, len(vars))
	for i, v := range vars {
		v.Key = strings.TrimSpace(v.Key)
		out[i] = EnvVar{Key: v.Key, Sensitive: v.Sensitive}
		if v.Key == "" {
			continue
		}
		if v.Sensitive {
			if v.Value == "" && v.ValueEnc != "" {
				out[i].ValueEnc = v.ValueEnc
				out[i].HasValue = true
				continue
			}
			if v.Value == "" {
				continue
			}
			if box == nil {
				return nil, fmt.Errorf("sensitive env %q requires encryption key", v.Key)
			}
			blob, err := box.Seal([]byte(v.Value))
			if err != nil {
				return nil, err
			}
			out[i].ValueEnc = hex.EncodeToString(blob)
			out[i].HasValue = true
			continue
		}
		out[i].Value = v.Value
		out[i].HasValue = v.Value != ""
	}
	return out, nil
}

// RedactApps strips secrets for API responses.
func RedactApps(apps []SpecApp) []SpecApp {
	out := make([]SpecApp, len(apps))
	for i, a := range apps {
		out[i] = a
		out[i].Env = RedactEnv(a.Env)
	}
	return out
}

// RedactEnv clears sensitive plaintext and ciphertext from a response payload.
func RedactEnv(vars []EnvVar) []EnvVar {
	if vars == nil {
		return nil
	}
	out := make([]EnvVar, len(vars))
	for i, v := range vars {
		out[i] = EnvVar{
			Key:       v.Key,
			Sensitive: v.Sensitive,
			HasValue:  v.Value != "" || v.ValueEnc != "" || v.HasValue,
		}
		if !v.Sensitive {
			out[i].Value = v.Value
		}
	}
	return out
}

// RedactProject returns a copy safe to JSON to the browser.
func RedactProject(p Project) Project {
	p.Apps = RedactApps(p.Apps)
	return p
}

// MergeAppsEnv seals incoming apps, preserving existing sensitive ciphertext when
// the client sends a blank value with has_value (replace-only semantics).
func MergeAppsEnv(box *cryptobox.Box, oldApps, incoming []SpecApp) ([]SpecApp, error) {
	byName := map[string]SpecApp{}
	for _, a := range oldApps {
		byName[a.Name] = a
	}
	out := make([]SpecApp, len(incoming))
	for i, in := range incoming {
		prev := byName[in.Name]
		merged := in
		env, err := mergeEnvList(box, prev.Env, in.Env)
		if err != nil {
			return nil, err
		}
		merged.Env = env
		out[i] = merged
	}
	return out, nil
}

func mergeEnvList(box *cryptobox.Box, old, incoming []EnvVar) ([]EnvVar, error) {
	oldByKey := map[string]EnvVar{}
	for _, v := range old {
		oldByKey[v.Key] = v
	}
	prepared := make([]EnvVar, 0, len(incoming))
	for _, v := range incoming {
		v.Key = strings.TrimSpace(v.Key)
		if v.Key == "" {
			continue
		}
		if v.Sensitive && v.Value == "" {
			if prev, ok := oldByKey[v.Key]; ok && prev.Sensitive && prev.ValueEnc != "" {
				v.ValueEnc = prev.ValueEnc
			}
		}
		prepared = append(prepared, v)
	}
	return sealEnv(box, prepared)
}

// ResolveAppEnv decrypts an app's env into a plain map for the agent.
func ResolveAppEnv(box *cryptobox.Box, app SpecApp) (map[string]string, error) {
	out := map[string]string{}
	for _, v := range app.Env {
		if v.Key == "" {
			continue
		}
		if v.Sensitive {
			if v.ValueEnc == "" {
				continue
			}
			if box == nil {
				return nil, fmt.Errorf("sensitive env %q: no encryption key", v.Key)
			}
			raw, err := hex.DecodeString(v.ValueEnc)
			if err != nil {
				return nil, fmt.Errorf("env %q: %w", v.Key, err)
			}
			plain, err := box.Open(raw)
			if err != nil {
				return nil, fmt.Errorf("env %q: %w", v.Key, err)
			}
			out[v.Key] = string(plain)
			continue
		}
		out[v.Key] = v.Value
	}
	return out, nil
}

// EnvMapFromVars is a convenience for non-sensitive-only maps (legacy project.env).
func EnvMapFromVars(vars []EnvVar) map[string]string {
	out := map[string]string{}
	for _, v := range vars {
		if v.Key == "" || v.Sensitive {
			continue
		}
		out[v.Key] = v.Value
	}
	return out
}
