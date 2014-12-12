package procfs

import (
	"fmt"
	"os"
	"path"
	"strconv"
)

// ProcProcess provides information about a running process.
type ProcProcess struct {
	// The process ID.
	PID int

	fs *ProcFS
}

// Self returns a process for the current process.
func Self() (*ProcProcess, error) {
	return Process(os.Getpid())
}

// Process returns a process for the given pid under /proc.
func Process(pid int) (*ProcProcess, error) {
	fs, err := NewFS(DefaultMountPoint)
	if err != nil {
		return nil, err
	}

	return fs.Process(pid)
}

// Process returns a process for the given pid.
func (fs *ProcFS) Process(pid int) (*ProcProcess, error) {
	if _, err := fs.stat(strconv.Itoa(pid)); err != nil {
		return nil, err
	}

	return &ProcProcess{PID: pid, fs: fs}, nil
}

// FileDescriptors returns the currently open file descriptors of a process.
func (p *ProcProcess) FileDescriptors() ([]uintptr, error) {
	names, err := p.fileDescriptors()
	if err != nil {
		return nil, err
	}

	fds := make([]uintptr, len(names))
	for i, n := range names {
		fd, err := strconv.ParseInt(n, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("could not parse fd %s: %s", n, err)
		}
		fds[i] = uintptr(fd)
	}

	return fds, nil
}

// FileDescriptorsLen returns the number of currently open file descriptors of
// a process.
func (p *ProcProcess) FileDescriptorsLen() (int, error) {
	fds, err := p.fileDescriptors()
	if err != nil {
		return 0, err
	}

	return len(fds), nil
}

func (p *ProcProcess) fileDescriptors() ([]string, error) {
	d, err := p.open("fd")
	if err != nil {
		return nil, err
	}
	defer d.Close()

	names, err := d.Readdirnames(-1)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %s", d.Name(), err)
	}

	return names, nil
}

func (p *ProcProcess) open(pa string) (*os.File, error) {
	if p.fs == nil {
		return nil, fmt.Errorf("missing procfs")
	}
	return p.fs.open(path.Join(strconv.Itoa(p.PID), pa))
}
