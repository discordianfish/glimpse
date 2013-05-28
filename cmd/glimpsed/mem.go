package main

import (
	"path"
	"sync"
)

type Op string

const (
	Add Op = "add"
	Del Op = "del"
)

type Change interface {
	Operation() Op
	Service() *Service
}

type Ref struct {
	*Job
	Rev int64
}

type Store interface {
	Put(Ref) (Ref, error)
	Get(Path) (*Ref, error)
	Match(ServiceAddress, WatchFunc) ([]Service, error)
}

type change struct {
	op  Op
	srv *Service
}

func (c change) Operation() Op     { return c.op }
func (c change) Service() *Service { return c.srv }

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

	for match, funcs := range s.watches {
		alive = alive[:0]
		if ok, _ := path.Match(string(match), string(addr)); ok {
			for _, handler := range funcs {
				if handler(ch) {
					alive = append(alive, handler)
				}
			}
		}

		copy(funcs, alive)
		funcs = funcs[:len(alive)]
		s.watches[match] = funcs
	}
}

// Holds write lock
func (s Mem) notify(del, add *Job) {
	dels := del.Services()
	adds := add.Services()

	var i, j int
	for i < len(dels) && j < len(adds) {
		switch {
		case dels[i].String() < adds[j].String():
			s.broadcast(change{Del, &dels[i]})
			i++
		case dels[i].String() > adds[j].String():
			s.broadcast(change{Add, &dels[i]})
			j++
		default:
			i++
			j++
		}
	}
	for ; i < len(dels); i++ {
		s.broadcast(change{Del, &dels[i]})
	}
	for ; j < len(adds); j++ {
		s.broadcast(change{Add, &adds[i]})
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

type WatchFunc func(Change) bool

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
