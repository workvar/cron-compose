// Package procgroup ties a child process and everything it spawns to one handle, so
// a timeout or cancel can stop the whole tree and resource caps apply to all of it.
//
// On Windows this is a Job Object: killing bash.exe alone would leave the npm or
// python it started running, and Job Objects are also how Windows enforces CPU,
// memory, and process-count limits (the systemd-run equivalent). On Unix the
// existing mechanisms (CommandContext, systemd-run scopes) are unchanged and a
// Group is a no-op.
package procgroup

// Limits are the caps a Group enforces. Zero means unlimited.
type Limits struct {
	CPUQuotaPercent int // 100 = one full core
	MemoryMaxMB     int
	TasksMax        int // maximum live processes in the tree
}

// Any reports whether at least one cap is set.
func (l Limits) Any() bool {
	return l.CPUQuotaPercent > 0 || l.MemoryMaxMB > 0 || l.TasksMax > 0
}
