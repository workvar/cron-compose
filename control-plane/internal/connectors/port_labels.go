package connectors

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const maxPortLabelRunes = 64

// NormalizeLabel trims a user-supplied port label. An empty result means delete
// the stored label. Labels longer than 64 runes are rejected.
func NormalizeLabel(raw string) (label string, del bool, err error) {
	label = strings.TrimSpace(raw)
	if strings.ContainsAny(label, "\n\r") {
		return "", false, fmt.Errorf("label cannot contain newlines")
	}
	if utf8.RuneCountInString(label) > maxPortLabelRunes {
		return "", false, fmt.Errorf("label must be at most %d characters", maxPortLabelRunes)
	}
	if label == "" {
		return "", true, nil
	}
	return label, false, nil
}

func portLabelKey(serverID, proto, address string, port int) string {
	return serverID + "\x00" + strings.ToLower(proto) + "\x00" + address + "\x00" + fmt.Sprint(port)
}

// ApplyLabels copies rows and fills Label from matching stored labels for serverID.
func ApplyLabels(serverID string, rows []PortRow, labels []PortLabel) []PortRow {
	byKey := make(map[string]string, len(labels))
	for _, l := range labels {
		if l.ServerID != serverID {
			continue
		}
		byKey[portLabelKey(l.ServerID, l.Proto, l.Address, l.Port)] = l.Label
	}
	out := make([]PortRow, len(rows))
	copy(out, rows)
	for i := range out {
		out[i].Label = byKey[portLabelKey(serverID, out[i].Proto, out[i].Address, out[i].Port)]
	}
	return out
}
