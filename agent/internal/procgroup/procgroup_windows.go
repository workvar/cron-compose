//go:build windows

package procgroup

import (
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Job Object CPU rate control is not wrapped by x/sys/windows.
const (
	jobObjectCPURateControlInformation = 15
	cpuRateControlEnable               = 0x1
	cpuRateControlHardCap              = 0x4
)

type cpuRateControlInformation struct {
	ControlFlags uint32
	CPURate      uint32 // 1/100ths of a percent of total machine CPU
}

// Group wraps a Job Object holding one process tree.
type Group struct {
	job windows.Handle
}

// Supported reports whether Attach enforces limits on this platform.
func Supported() bool { return true }

// Attach creates a Job Object with lim and moves p into it. Everything p starts
// afterwards inherits the job, so Kill reaches grandchildren too. Close only drops
// the agent's handle: like on Linux, a background process a job deliberately left
// running keeps running (still under the job's caps).
func Attach(p *os.Process, lim Limits) (*Group, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create job object: %w", err)
	}
	g := &Group{job: job}
	if err := g.setLimits(lim); err != nil {
		g.Close()
		return nil, err
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(p.Pid))
	if err != nil {
		g.Close()
		return nil, fmt.Errorf("open process %d: %w", p.Pid, err)
	}
	defer windows.CloseHandle(h)
	if err := windows.AssignProcessToJobObject(job, h); err != nil {
		g.Close()
		return nil, fmt.Errorf("assign process to job: %w", err)
	}
	return g, nil
}

func (g *Group) setLimits(lim Limits) error {
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	if lim.MemoryMaxMB > 0 {
		info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_JOB_MEMORY
		info.JobMemoryLimit = uintptr(lim.MemoryMaxMB) << 20
	}
	if lim.TasksMax > 0 {
		info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS
		info.BasicLimitInformation.ActiveProcessLimit = uint32(lim.TasksMax)
	}
	if _, err := windows.SetInformationJobObject(g.job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		return fmt.Errorf("set job limits: %w", err)
	}
	if lim.CPUQuotaPercent <= 0 {
		return nil
	}
	cpu := cpuRateControlInformation{
		ControlFlags: cpuRateControlEnable | cpuRateControlHardCap,
		CPURate:      CPURate(lim.CPUQuotaPercent, runtime.NumCPU()),
	}
	if _, err := windows.SetInformationJobObject(g.job, jobObjectCPURateControlInformation,
		uintptr(unsafe.Pointer(&cpu)), uint32(unsafe.Sizeof(cpu))); err != nil {
		return fmt.Errorf("set job cpu rate: %w", err)
	}
	return nil
}

// Kill terminates every process in the group.
func (g *Group) Kill() error {
	if g == nil || g.job == 0 {
		return nil
	}
	return windows.TerminateJobObject(g.job, 1)
}

// Close releases the agent's handle to the job.
func (g *Group) Close() error {
	if g == nil || g.job == 0 {
		return nil
	}
	err := windows.CloseHandle(g.job)
	g.job = 0
	return err
}
