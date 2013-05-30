package main

import (
	"path"
	"sort"
	"sync"
)

type Mem struct {
	m       sync.RWMutex
	jobs    map[Path]*Job
	rev     int64
	watches map[ServiceAddress][]WatchFunc
}

func newMemStore() *Mem {
	const transactionLogSize = 100000

	m := &Mem{
		jobs:    make(map[Path]*Job),
		watches: make(map[ServiceAddress][]WatchFunc),
	}
	return m
}

func (s Mem) broadcast(ch Change) {
	addr := ch.Service().Address()
	alive := make([]WatchFunc, 0)

	for watch, funcs := range s.watches {
		alive = alive[:0]
		if watch.Match(addr) {
			for _, handler := range funcs {
				if handler(ch) {
					alive = append(alive, handler)
				}
			}
		}

		copy(funcs, alive)
		funcs = funcs[:len(alive)]
		s.watches[watch] = funcs
	}
}

// Holds write lock
func (s Mem) notify(del, add *Job) {
	for _, c := range del.Diff(add) {
		s.broadcast(c)
	}
}

// Holds write lock
func (s Mem) listen(addr ServiceAddress, fn WatchFunc) {
	s.watches[addr] = append(s.watches[addr], fn)
}

func (s Mem) Put(ref Ref) (Ref, error) {
	s.m.Lock()
	defer s.m.Unlock()

	path := ref.Job.Path()
	old := Ref{s.jobs[path], s.rev}

	if ref.Rev > s.rev {
		s.rev = ref.Rev
	}
	s.rev++
	s.jobs[path] = ref.Job

	s.notify(old.Job, ref.Job)

	return Ref{ref.Job, s.rev}, nil
}

func (s Mem) Get(path Path) (*Ref, error) {
	s.m.RLock()
	defer s.m.RUnlock()
	return &Ref{s.jobs[path], s.rev}, nil
}

func (s Mem) Match(glob ServiceAddress, watch WatchFunc) ([]Service, error) {
	s.m.Lock()
	defer s.m.Unlock()

	var srvs []Service

	for key, job := range s.jobs {
		matched, err := path.Match((glob.JobPath()), string(key))
		if err != nil {
			return nil, err
		}

		if matched {
			for _, srv := range job.Services() {
				if matched, _ = path.Match(string(glob), string(srv.Address())); matched {
					srvs = append(srvs, srv)
				}
			}
		}
	}

	if watch != nil {
		s.listen(glob, watch)
	}

	sort.Sort(ServiceGroup(srvs))

	return srvs, nil
}

/*
func (s Mem) Wait(glob Path, instanceIndex, endpointName string, rev int64) ([]Op, int64, error) {
	w := wait{rev: rev, res: make(chan Op)}
	s.waits <- w

	var ops []Op
	for op := range w.res {
		ops = append(ops, op)
	}

	return ops, s.rev, nil
}
*/
