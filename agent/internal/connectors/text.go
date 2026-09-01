package connectors

import (
	"errors"
	"fmt"
	"strings"
)

// containsAny reports whether s contains any of the needles, case-insensitively.
// Used to classify tool output, which is not a stable interface, so matching is
// deliberately loose.
func containsAny(s string, needles ...string) bool {
	low := strings.ToLower(s)
	for _, n := range needles {
		if strings.Contains(low, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

// balancedBraces is the only structural check we can honestly make on a config
// fragment we cannot hand to the tool itself. It catches the overwhelmingly common
// editing mistake (a dropped closing brace) without pretending to be a parser.
//
// Quoted strings and # comments are skipped so a brace inside either does not count.
func balancedBraces(content []byte) error {
	depth, line := 0, 1
	var quote byte
	comment := false

	for i := 0; i < len(content); i++ {
		c := content[i]
		switch {
		case c == '\n':
			line++
			comment = false
		case comment:
			// skip
		case quote != 0:
			if c == '\\' {
				i++ // skip the escaped character
			} else if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '#':
			comment = true
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth < 0 {
				return fmt.Errorf("unexpected '}' on line %d", line)
			}
		}
	}
	if quote != 0 {
		return errors.New("unterminated quoted string")
	}
	if depth > 0 {
		return fmt.Errorf("%d unclosed '{' block(s)", depth)
	}
	return nil
}
