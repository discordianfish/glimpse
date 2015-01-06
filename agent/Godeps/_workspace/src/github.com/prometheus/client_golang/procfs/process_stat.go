package procfs

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
)

// #include <unistd.h>
import "C"

// ProcessStat provides status information about the process,
// read from /proc/[pid]/stat.
type ProcessStat struct {
	// The process ID.
	PID int
	// The filename of the executable.
	Comm string
	// The process state.
	State string
	// The PID of the parent of this process.
	PPID int
	// The process group ID of the process.
	PGRP int
	// The session ID of the process.
	Session int
	// The controlling terminal of the process.
	TTY int
	// The ID of the foreground process group of the controlling terminal of
	// the process.
	TPGID int
	// The kernel flags word of the process.
	Flags uint
	// The number of minor faults the process has made which have not required
	// loading a memory page from disk.
	MinFlt uint
	// The number of minor faults that the process's waited-for children have
	// made.
	CMinFlt uint
	// The number of major faults the process has made which have required
	// loading a memory page from disk.
	MajFlt uint
	// The number of major faults that the process's waited-for children have
	// made.
	CMajFlt uint
	// Amount of time that this process has been scheduled in user mode,
	// measured in clock ticks.
	UTime uint
	// Amount of time that this process has been scheduled in kernel mode,
	// measured in clock ticks.
	STime uint
	// Amount of time that this process's waited-for children have been
	// scheduled in user mode, measured in clock ticks.
	CUTime uint
	// Amount of time that this process's waited-for children have been
	// scheduled in kernel mode, measured in clock ticks.
	CSTime uint
	// For processes running a real-time scheduling policy, this is the negated
	// scheduling priority, minus one.
	Priority int
	// The nice value, a value in the range 19 (low priority) to -20 (high
	// priority).
	Nice int
	// Number of threads in this process.
	NumThreads int
	// The time the process started after system boot, the value is expressed
	// in clock ticks.
	Starttime uint64
	// Virtual memory size in bytes.
	VSize int
	// Resident set size in pages.
	RSS int

	fs *ProcFS
}

// Stat returns the current status information of the process.
func (p *ProcProcess) Stat() (*ProcessStat, error) {
	f, err := p.open("stat")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	var (
		ignore int

		s = ProcessStat{PID: p.PID, fs: p.fs}
		l = bytes.Index(data, []byte("("))
		r = bytes.LastIndex(data, []byte(")"))
	)

	if l < 0 || r < 0 {
		return nil, fmt.Errorf(
			"unexpected format, couldn't extract comm: %s",
			data,
		)
	}

	s.Comm = string(data[l+1 : r])
	_, err = fmt.Fscan(
		bytes.NewBuffer(data[r+2:]),
		&s.State,
		&s.PPID,
		&s.PGRP,
		&s.Session,
		&s.TTY,
		&s.TPGID,
		&s.Flags,
		&s.MinFlt,
		&s.CMinFlt,
		&s.MajFlt,
		&s.CMajFlt,
		&s.UTime,
		&s.STime,
		&s.CUTime,
		&s.CSTime,
		&s.Priority,
		&s.Nice,
		&s.NumThreads,
		&ignore,
		&s.Starttime,
		&s.VSize,
		&s.RSS,
	)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

// VirtualMemory returns the virtual memory size in bytes.
func (s *ProcessStat) VirtualMemory() int {
	return s.VSize
}

// ResidentMemory returns the resident memory size in bytes.
func (s *ProcessStat) ResidentMemory() int {
	return s.RSS * os.Getpagesize()
}

// StartTime returns the unix timestamp of the process in seconds.
func (s *ProcessStat) StartTime() (float64, error) {
	if s.fs == nil {
		return 0, fmt.Errorf("missing procfs")
	}
	stat, err := s.fs.Stat()
	if err != nil {
		return 0, err
	}
	return float64(stat.BootTime) + (float64(s.Starttime) / ticks()), nil
}

// CPUTime returns the total CPU user and system time in seconds.
func (s *ProcessStat) CPUTime() float64 {
	return float64(s.UTime+s.STime) / ticks()
}

func ticks() float64 {
	return float64(C.sysconf(C._SC_CLK_TCK)) // most likely 100
}
