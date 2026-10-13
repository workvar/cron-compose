package deploy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var ecosystemBasenames = []string{
	"ecosystem.config.js", "ecosystem.config.cjs", "ecosystem.config.mjs",
	"ecosystem.config.ts", "pm2.json",
}

// findEcosystem returns the ecosystem / pm2.json path for an app work dir.
// It checks the app folder first, then the release root (parent) — monorepos
// often keep one ecosystem at current/ with per-app cwd entries.
func findEcosystem(dir string) string {
	dir = filepath.Clean(dir)
	for _, base := range []string{dir, filepath.Dir(dir)} {
		if base == "" || base == "." || base == string(filepath.Separator) {
			continue
		}
		for _, name := range ecosystemBasenames {
			p := filepath.Join(base, name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p
			}
		}
	}
	return ""
}

// ecosystemNamesForWork returns apps[].name values from the ecosystem that
// belong to this work directory (cwd match or single-app file).
func ecosystemNamesForWork(ecoPath, work string) []string {
	if ecoPath == "" {
		return nil
	}
	body, err := os.ReadFile(ecoPath)
	if err != nil {
		return nil
	}
	apps := parseEcosystemApps(string(body))
	if len(apps) == 0 {
		return nil
	}
	work = resolvePath(work)
	base := filepath.Base(work)
	ecoDir := filepath.Dir(ecoPath)
	var names []string
	for _, a := range apps {
		if a.Name == "" {
			continue
		}
		appDir := ecoDir
		if cwd := strings.TrimSpace(a.Cwd); cwd != "" && cwd != "." {
			appDir = filepath.Join(ecoDir, cwd)
		}
		appDir = resolvePath(appDir)
		if appDir == work || filepath.Base(appDir) == base || sameWorkDir(appDir, work, base) {
			names = append(names, a.Name)
			continue
		}
		// Ecosystem lives inside the app folder — every entry belongs here.
		if resolvePath(ecoDir) == work {
			names = append(names, a.Name)
		}
	}
	if len(names) == 0 && len(apps) == 1 && apps[0].Name != "" {
		// Single-app ecosystem next to (or above) the work dir.
		names = append(names, apps[0].Name)
	}
	return names
}

// preferEcosystemName picks the host process name: explicit override wins, else
// an ecosystem apps[].name that matches this app folder, else the qualified want.
func preferEcosystemName(want, work, ecoPath string) string {
	names := ecosystemNamesForWork(ecoPath, work)
	if len(names) == 0 {
		return want
	}
	base := filepath.Base(work)
	for _, n := range names {
		if pm2NameMatch(n, want, base) || n == want {
			return n
		}
	}
	return names[0]
}

type ecoApp struct {
	Name string
	Cwd  string
}

func parseEcosystemApps(body string) []ecoApp {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	if apps := parseEcoJSON(body); len(apps) > 0 {
		return apps
	}
	return parseEcoRegex(body)
}

func parseEcoJSON(body string) []ecoApp {
	// Strip a leading module.exports / export default assignment for near-JSON files.
	cleaned := body
	if i := strings.Index(cleaned, "{"); i >= 0 {
		if j := strings.LastIndex(cleaned, "}"); j > i {
			candidate := cleaned[i : j+1]
			candidate = rewriteEcoQuotes(candidate)
			if apps := decodeEcoApps(candidate); len(apps) > 0 {
				return apps
			}
		}
	}
	return decodeEcoApps(body)
}

func decodeEcoApps(body string) []ecoApp {
	var wrap struct {
		Apps json.RawMessage `json:"apps"`
	}
	if err := json.Unmarshal([]byte(body), &wrap); err != nil || len(wrap.Apps) == 0 {
		return nil
	}
	var one struct {
		Name string `json:"name"`
		Cwd  string `json:"cwd"`
	}
	if err := json.Unmarshal(wrap.Apps, &one); err == nil && one.Name != "" {
		return []ecoApp{{Name: one.Name, Cwd: one.Cwd}}
	}
	var list []struct {
		Name string `json:"name"`
		Cwd  string `json:"cwd"`
	}
	if err := json.Unmarshal(wrap.Apps, &list); err != nil {
		return nil
	}
	out := make([]ecoApp, 0, len(list))
	for _, a := range list {
		if a.Name == "" {
			continue
		}
		out = append(out, ecoApp{Name: a.Name, Cwd: a.Cwd})
	}
	return out
}

func rewriteEcoQuotes(s string) string {
	// Quote bare keys and turn single-quoted strings into double-quoted.
	keyRe := regexp.MustCompile(`([{\[,]\s*)([A-Za-z_][A-Za-z0-9_]*)\s*:`)
	s = keyRe.ReplaceAllString(s, `$1"$2":`)
	s = strings.ReplaceAll(s, `'`, `"`)
	trail := regexp.MustCompile(`,(\s*[}\]])`)
	return trail.ReplaceAllString(s, `$1`)
}

var (
	ecoNameRe = regexp.MustCompile(`(?i)\bname\s*:\s*['"]([^'"]+)['"]`)
	ecoCwdRe  = regexp.MustCompile(`(?i)\bcwd\s*:\s*['"]([^'"]+)['"]`)
)

func parseEcoRegex(body string) []ecoApp {
	names := ecoNameRe.FindAllStringSubmatch(body, -1)
	if len(names) == 0 {
		return nil
	}
	cwds := ecoCwdRe.FindAllStringSubmatch(body, -1)
	out := make([]ecoApp, 0, len(names))
	for i, m := range names {
		a := ecoApp{Name: m[1]}
		if i < len(cwds) {
			a.Cwd = cwds[i][1]
		}
		out = append(out, a)
	}
	return out
}
