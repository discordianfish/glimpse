package main

import (
	"path"
)

type mem struct {
	jobs map[Path]*Job
	rev  int64
}

func newMemStore() *mem {
	return &mem{
		jobs: make(map[Path]*Job),
	}
}

func (s mem) Put(ref Ref) (Ref, error) {
	s.jobs[ref.Job.Path()] = ref.Job
	if ref.Rev > s.rev {
		s.rev = ref.Rev
	}
	s.rev++
	return Ref{ref.Job, s.rev}, nil
}

func (s mem) Get(path Path) (*Ref, error) {
	return &Ref{s.jobs[path], s.rev}, nil
}

func (s mem) Glob(glob Path) ([]Ref, error) {
	var res []Ref
	for key, job := range s.jobs {
		ok, err := path.Match(string(glob), string(key))
		if err != nil {
			return nil, err
		}
		if ok {
			res = append(res, Ref{job, s.rev})
		}
	}
	return res, nil
}
