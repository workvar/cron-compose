package setup

import "strings"

// ParseMode reads DB_BOOTSTRAP env values.
func ParseMode(raw string) Mode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "docker":
		return ModeDocker
	case "none", "off", "0", "false":
		return ModeNone
	default:
		return ModeLocal
	}
}
