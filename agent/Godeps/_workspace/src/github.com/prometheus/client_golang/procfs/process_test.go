package procfs

import (
	"os"
	"reflect"
	"testing"
)

func TestSelf(t *testing.T) {
	p1, err := Process(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	p2, err := Self()
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(p1, p2) {
		t.Errorf("want process %v to equal %v", p1, p2)
	}
}

func TestFileDescriptors(t *testing.T) {
	p1, err := testProcess(26231)
	if err != nil {
		t.Fatal(err)
	}
	fds, err := p1.FileDescriptors()
	if err != nil {
		t.Fatal(err)
	}

	if want := []uintptr{2, 4, 1, 3, 0}; !reflect.DeepEqual(want, fds) {
		t.Errorf("want fds %v, got %v", want, fds)
	}

	p2, err := Self()
	if err != nil {
		t.Fatal(err)
	}

	fdsBefore, err := p2.FileDescriptors()
	if err != nil {
		t.Fatal(err)
	}

	s, err := os.Open("fixtures")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	fdsAfter, err := p2.FileDescriptors()
	if err != nil {
		t.Fatal(err)
	}

	if len(fdsBefore)+1 != len(fdsAfter) {
		t.Errorf("want fds %v+1 to equal %v", fdsBefore, fdsAfter)
	}
}

func TestFileDescriptorsLen(t *testing.T) {
	p1, err := testProcess(26231)
	if err != nil {
		t.Fatal(err)
	}
	l, err := p1.FileDescriptorsLen()
	if err != nil {
		t.Fatal(err)
	}
	if want, got := 5, l; want != got {
		t.Errorf("want fds %d, got %d", want, got)
	}
}

func testProcess(pid int) (*ProcProcess, error) {
	fs, err := NewFS("fixtures")
	if err != nil {
		return nil, err
	}

	return fs.Process(pid)
}
