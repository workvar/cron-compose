package scheduler

import "time"

// LoadLocation returns the IANA timezone or UTC if the name is unknown. Exported so
// the catch-up calculation resolves a job's timezone exactly the way the scheduler
// does; a mismatch there would compute the wrong missed windows around a DST change.
func LoadLocation(name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	if loc, err := time.LoadLocation(name); err == nil {
		return loc
	}
	return time.UTC
}

// loadLocation is the internal alias the scheduler itself uses.
func loadLocation(name string) *time.Location { return LoadLocation(name) }
