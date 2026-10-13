package deploys

import (
	"encoding/json"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// PM2AppHint is one apps[] entry extracted from an ecosystem / pm2.json file.
type PM2AppHint struct {
	Name   string `json:"name"`
	Script string `json:"script,omitempty"`
	Args   string `json:"args,omitempty"`
	Cwd    string `json:"cwd,omitempty"`
	Port   int    `json:"port,omitempty"`
	// File is the repo-relative path of the ecosystem file this came from.
	File string `json:"file,omitempty"`
}

var ecosystemFileNames = map[string]bool{
	"ecosystem.config.js":  true,
	"ecosystem.config.cjs": true,
	"ecosystem.config.mjs": true,
	"ecosystem.config.ts":  true,
	"pm2.json":             true,
}

// CollectPM2Apps finds ecosystem files in the tree and returns apps that belong
// to root ("" / "." = whole repo). An app matches when its effective directory
// (ecosystem dir + cwd) equals the scoped root, or when the ecosystem lives in
// that root and cwd is empty.
func CollectPM2Apps(files map[string]string, root string) []PM2AppHint {
	root = normalizeEcoRoot(root)
	var out []PM2AppHint
	for p, body := range files {
		base := path.Base(p)
		if !ecosystemFileNames[base] {
			continue
		}
		ecoDir := normalizeEcoRoot(path.Dir(p))
		for _, app := range ParsePM2Ecosystem(body) {
			app.File = p
			appRoot := joinEcoRoot(ecoDir, app.Cwd)
			if !pm2AppMatchesRoot(root, ecoDir, appRoot) {
				continue
			}
			// Surface cwd relative to the scoped root for the UI.
			if root != "." && strings.HasPrefix(appRoot+"/", root+"/") {
				app.Cwd = strings.TrimPrefix(appRoot, root+"/")
			} else if appRoot == root {
				app.Cwd = "."
			} else {
				app.Cwd = appRoot
			}
			out = append(out, app)
		}
	}
	return out
}

func pm2AppMatchesRoot(scope, ecoDir, appRoot string) bool {
	if scope == "." {
		// Unscoped inspect: every app in the tree.
		return true
	}
	// Ecosystem inside the scoped folder (typical per-app ecosystem.config.js).
	if ecoDir == scope || strings.HasPrefix(ecoDir+"/", scope+"/") {
		return true
	}
	// Root-level ecosystem with cwd pointing at this block.
	return appRoot == scope
}

func normalizeEcoRoot(root string) string {
	root = strings.Trim(strings.TrimSpace(strings.ReplaceAll(root, "\\", "/")), "/")
	if root == "" || root == "." {
		return "."
	}
	return root
}

func joinEcoRoot(dir, cwd string) string {
	cwd = strings.Trim(strings.TrimSpace(strings.ReplaceAll(cwd, "\\", "/")), "/")
	if cwd == "" || cwd == "." {
		return dir
	}
	if dir == "." {
		return cwd
	}
	return path.Join(dir, cwd)
}

// ParsePM2Ecosystem extracts apps from pm2.json or a CommonJS/ESM ecosystem file.
// It is intentionally tolerant: comments, single quotes, and trailing commas are
// stripped before a JSON pass; a regex fallback recovers name/script/cwd/port.
func ParsePM2Ecosystem(body string) []PM2AppHint {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	if apps := parsePM2JSON(body); len(apps) > 0 {
		return apps
	}
	cleaned := stripJSComments(body)
	obj := extractEcosystemObject(cleaned)
	if obj != "" {
		if apps := parsePM2JSON(jsObjectToJSON(obj)); len(apps) > 0 {
			return apps
		}
	}
	return parsePM2AppsRegex(cleaned)
}

func parsePM2JSON(body string) []PM2AppHint {
	var wrap struct {
		Apps json.RawMessage `json:"apps"`
	}
	if err := json.Unmarshal([]byte(body), &wrap); err != nil || len(wrap.Apps) == 0 {
		// Bare array.
		var list []pm2AppRaw
		if err := json.Unmarshal([]byte(body), &list); err != nil {
			return nil
		}
		return normalizePM2Raws(list)
	}
	// apps may be a single object or an array.
	var one pm2AppRaw
	if err := json.Unmarshal(wrap.Apps, &one); err == nil && (one.Name != "" || one.Script != "") {
		return normalizePM2Raws([]pm2AppRaw{one})
	}
	var list []pm2AppRaw
	if err := json.Unmarshal(wrap.Apps, &list); err != nil {
		return nil
	}
	return normalizePM2Raws(list)
}

type pm2AppRaw struct {
	Name   string          `json:"name"`
	Script string          `json:"script"`
	Args   json.RawMessage `json:"args"`
	Cwd    string          `json:"cwd"`
	Env    map[string]any  `json:"env"`
	EnvProd map[string]any `json:"env_production"`
}

func normalizePM2Raws(list []pm2AppRaw) []PM2AppHint {
	out := make([]PM2AppHint, 0, len(list))
	for _, raw := range list {
		h := PM2AppHint{
			Name:   strings.TrimSpace(raw.Name),
			Script: strings.TrimSpace(raw.Script),
			Args:   stringifyPM2Args(raw.Args),
			Cwd:    strings.TrimSpace(raw.Cwd),
			Port:   portFromEnvMap(raw.Env),
		}
		if h.Port == 0 {
			h.Port = portFromEnvMap(raw.EnvProd)
		}
		if h.Name == "" && h.Script == "" {
			continue
		}
		out = append(out, h)
	}
	return out
}

func stringifyPM2Args(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return strings.TrimSpace(strings.Join(arr, " "))
	}
	var anyArr []any
	if err := json.Unmarshal(raw, &anyArr); err == nil {
		parts := make([]string, 0, len(anyArr))
		for _, v := range anyArr {
			parts = append(parts, stringifyAny(v))
		}
		return strings.TrimSpace(strings.Join(parts, " "))
	}
	return ""
}

func stringifyAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func portFromEnvMap(env map[string]any) int {
	if env == nil {
		return 0
	}
	v, ok := env["PORT"]
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	case float64:
		return int(t)
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	default:
		return 0
	}
}

func stripJSComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inS, inD, inLine, inBlock := false, false, false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inLine {
			if c == '\n' {
				inLine = false
				b.WriteByte(c)
			}
			continue
		}
		if inBlock {
			if c == '*' && i+1 < len(s) && s[i+1] == '/' {
				inBlock = false
				i++
			}
			continue
		}
		if inS {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				b.WriteByte(s[i+1])
				i++
				continue
			}
			if c == '\'' {
				inS = false
			}
			continue
		}
		if inD {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				b.WriteByte(s[i+1])
				i++
				continue
			}
			if c == '"' {
				inD = false
			}
			continue
		}
		if c == '/' && i+1 < len(s) {
			switch s[i+1] {
			case '/':
				inLine = true
				i++
				continue
			case '*':
				inBlock = true
				i++
				continue
			}
		}
		if c == '\'' {
			inS = true
			b.WriteByte(c)
			continue
		}
		if c == '"' {
			inD = true
			b.WriteByte(c)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

var (
	moduleExportsRe = regexp.MustCompile(`(?s)(?:module\.exports\s*=\s*|export\s+default\s+)`)
)

func extractEcosystemObject(s string) string {
	loc := moduleExportsRe.FindStringIndex(s)
	start := 0
	if loc != nil {
		start = loc[1]
	}
	rest := strings.TrimSpace(s[start:])
	if rest == "" {
		return ""
	}
	if rest[0] != '{' {
		// Find first '{' after the assignment.
		i := strings.IndexByte(rest, '{')
		if i < 0 {
			return ""
		}
		rest = rest[i:]
	}
	depth := 0
	inS, inD := false, false
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if inS {
			if c == '\\' && i+1 < len(rest) {
				i++
				continue
			}
			if c == '\'' {
				inS = false
			}
			continue
		}
		if inD {
			if c == '\\' && i+1 < len(rest) {
				i++
				continue
			}
			if c == '"' {
				inD = false
			}
			continue
		}
		switch c {
		case '\'':
			inS = true
		case '"':
			inD = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return rest[:i+1]
			}
		}
	}
	return ""
}

// jsObjectToJSON turns a loose JS object literal into something json.Unmarshal
// can read: quoted keys, double-quoted strings, no trailing commas.
func jsObjectToJSON(s string) string {
	s = strings.TrimSpace(s)
	// Quote unquoted keys: name: → "name":
	keyRe := regexp.MustCompile(`([{\[,]\s*)([A-Za-z_][A-Za-z0-9_]*)\s*:`)
	s = keyRe.ReplaceAllString(s, `$1"$2":`)
	// Single-quoted strings → double-quoted.
	s = rewriteSingleQuotes(s)
	// Trailing commas before } or ].
	trail := regexp.MustCompile(`,(\s*[}\]])`)
	s = trail.ReplaceAllString(s, `$1`)
	return s
}

func rewriteSingleQuotes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inD := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inD {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				b.WriteByte(s[i+1])
				i++
				continue
			}
			if c == '"' {
				inD = false
			}
			continue
		}
		if c == '"' {
			inD = true
			b.WriteByte(c)
			continue
		}
		if c == '\'' {
			b.WriteByte('"')
			for i++; i < len(s); i++ {
				c = s[i]
				if c == '\\' && i+1 < len(s) {
					next := s[i+1]
					if next == '\'' {
						b.WriteByte('\'')
						i++
						continue
					}
					b.WriteByte(c)
					b.WriteByte(next)
					i++
					continue
				}
				if c == '\'' {
					b.WriteByte('"')
					break
				}
				if c == '"' {
					b.WriteString(`\"`)
					continue
				}
				b.WriteByte(c)
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

var (
	pm2NameRe   = regexp.MustCompile(`(?i)\bname\s*:\s*['"]([^'"]+)['"]`)
	pm2ScriptRe = regexp.MustCompile(`(?i)\bscript\s*:\s*['"]([^'"]+)['"]`)
	pm2CwdRe    = regexp.MustCompile(`(?i)\bcwd\s*:\s*['"]([^'"]+)['"]`)
	pm2PortRe   = regexp.MustCompile(`(?i)\bPORT\s*:\s*['"]?(\d+)['"]?`)
	pm2ArgsRe   = regexp.MustCompile(`(?i)\bargs\s*:\s*['"]([^'"]+)['"]`)
)

func parsePM2AppsRegex(body string) []PM2AppHint {
	// Split roughly on app boundaries (objects inside apps array).
	chunks := splitAppObjects(body)
	if len(chunks) == 0 {
		chunks = []string{body}
	}
	var out []PM2AppHint
	for _, chunk := range chunks {
		h := PM2AppHint{}
		if m := pm2NameRe.FindStringSubmatch(chunk); len(m) > 1 {
			h.Name = m[1]
		}
		if m := pm2ScriptRe.FindStringSubmatch(chunk); len(m) > 1 {
			h.Script = m[1]
		}
		if m := pm2CwdRe.FindStringSubmatch(chunk); len(m) > 1 {
			h.Cwd = m[1]
		}
		if m := pm2ArgsRe.FindStringSubmatch(chunk); len(m) > 1 {
			h.Args = m[1]
		}
		if m := pm2PortRe.FindStringSubmatch(chunk); len(m) > 1 {
			h.Port, _ = strconv.Atoi(m[1])
		}
		if h.Name == "" && h.Script == "" {
			continue
		}
		out = append(out, h)
	}
	return out
}

func splitAppObjects(body string) []string {
	idx := strings.Index(strings.ToLower(body), "apps")
	if idx < 0 {
		return nil
	}
	rest := body[idx:]
	bracket := strings.IndexByte(rest, '[')
	brace := strings.IndexByte(rest, '{')
	if bracket < 0 && brace < 0 {
		return nil
	}
	// Single object under apps: { name, script }
	if brace >= 0 && (bracket < 0 || brace < bracket) {
		obj := extractBalanced(rest[brace:], '{', '}')
		if obj != "" {
			return []string{obj}
		}
		return nil
	}
	arr := extractBalanced(rest[bracket:], '[', ']')
	if arr == "" {
		return nil
	}
	inner := strings.TrimSpace(arr[1 : len(arr)-1])
	var chunks []string
	for i := 0; i < len(inner); {
		for i < len(inner) && (inner[i] == ',' || inner[i] == ' ' || inner[i] == '\n' || inner[i] == '\t') {
			i++
		}
		if i >= len(inner) {
			break
		}
		if inner[i] != '{' {
			break
		}
		obj := extractBalanced(inner[i:], '{', '}')
		if obj == "" {
			break
		}
		chunks = append(chunks, obj)
		i += len(obj)
	}
	return chunks
}

func extractBalanced(s string, open, close byte) string {
	if s == "" || s[0] != open {
		return ""
	}
	depth := 0
	inS, inD := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inS {
			if c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == '\'' {
				inS = false
			}
			continue
		}
		if inD {
			if c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == '"' {
				inD = false
			}
			continue
		}
		switch c {
		case '\'':
			inS = true
		case '"':
			inD = true
		default:
			if c == open {
				depth++
			} else if c == close {
				depth--
				if depth == 0 {
					return s[:i+1]
				}
			}
		}
	}
	return ""
}

// RunFromPM2 builds a shell start command from script + args when present.
func RunFromPM2(app PM2AppHint) string {
	script := strings.TrimSpace(app.Script)
	args := strings.TrimSpace(app.Args)
	if script == "" {
		return ""
	}
	if args == "" {
		return script
	}
	return script + " " + args
}
