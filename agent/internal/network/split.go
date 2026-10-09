package network

import "strings"

// splitNM splits nmcli -t output on unescaped colons into at most n fields.
// When n <= 0, all fields are returned.
func splitNM(line string, n int) []string {
	var parts []string
	var b strings.Builder
	esc := false
	for _, r := range line {
		if esc {
			b.WriteRune(r)
			esc = false
			continue
		}
		if r == '\\' {
			esc = true
			continue
		}
		if r == ':' && (n <= 0 || len(parts) < n-1) {
			parts = append(parts, b.String())
			b.Reset()
			continue
		}
		b.WriteRune(r)
	}
	parts = append(parts, b.String())
	return parts
}
