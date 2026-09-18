//go:build !linux && !darwin

package hostmetrics

import "os"

func fillPlatform(s *Snapshot) {
	if h, err := os.Hostname(); err == nil {
		s.Hostname = h
	}
}
