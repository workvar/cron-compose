package deploy

import (
	"strings"
	"unicode"
)

// scrubPTYLog strips terminal control sequences from installer output so the
// deploy log UI shows readable text instead of npm's spinner frames
// (\x1b[1G\x1b[0K|/-\). Empty or spinner-only residue is dropped entirely.
func scrubPTYLog(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	s := stripANSI(string(data))
	s = strings.ReplaceAll(s, "\r", "")
	if strings.TrimSpace(s) == "" || spinnerOnly(s) {
		return nil
	}
	return []byte(s)
}

func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != 0x1b {
			b.WriteByte(s[i])
			continue
		}
		if i+1 >= len(s) {
			continue
		}
		switch s[i+1] {
		case '[': // CSI: ESC [ ... letter
			j := i + 2
			for j < len(s) && !unicode.IsLetter(rune(s[j])) && s[j] != '@' {
				j++
			}
			if j < len(s) {
				i = j
			} else {
				i = len(s) - 1
			}
		case ']': // OSC: ESC ] ... BEL or ST
			j := i + 2
			for j < len(s) && s[j] != 0x07 && !(s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\') {
				j++
			}
			if j < len(s) && s[j] == 0x1b {
				i = j + 1
			} else if j < len(s) {
				i = j
			} else {
				i = len(s) - 1
			}
		default:
			// ESC + single-char sequence (e.g. ESC c); drop both.
			i++
		}
	}
	return b.String()
}

func spinnerOnly(s string) bool {
	for _, r := range s {
		switch r {
		case '|', '/', '-', '\\', ' ', '\t', '\n':
		default:
			return false
		}
	}
	return true
}
