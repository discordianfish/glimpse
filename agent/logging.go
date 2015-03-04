package main

import (
	"log"
	"time"
)

type loggingStore struct {
	logger *log.Logger
	next   store
}

func newLoggingStore(logger *log.Logger, next store) *loggingStore {
	return &loggingStore{
		logger: logger,
		next:   next,
	}
}

func (s *loggingStore) getInstances(i info) (is instances, err error) {
	defer func(start time.Time) {
		s.log(start, "getInstances", err, i.addr())
	}(time.Now())

	return s.next.getInstances(i)
}

func (s *loggingStore) getServers(zone string) (is instances, err error) {
	defer func(start time.Time) {
		s.log(start, "getServers", err, zone)
	}(time.Now())

	return s.next.getServers(zone)
}

func (s *loggingStore) log(start time.Time, op string, err error, input string) {
	if err == nil {
		return
	}

	label := errToLabel[errUntracked]

	switch e := err.(type) {
	case *glimpseError:
		label = errToLabel[e.err]
	}

	s.logger.Printf("STORE %d %s %s %s", time.Since(start), op, label, input)
}
