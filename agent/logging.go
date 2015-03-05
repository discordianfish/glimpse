package main

import (
	"log"
	"time"

	"github.com/miekg/dns"
)

func dnsLoggingHandler(logger *log.Logger, next dns.Handler) dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		next.ServeDNS(&loggingWriter{
			ResponseWriter: w,
			logger:         logger,
			start:          time.Now(),
		}, r)
	}
}

type loggingWriter struct {
	dns.ResponseWriter

	logger *log.Logger
	start  time.Time
}

func (w *loggingWriter) WriteMsg(res *dns.Msg) error {
	var (
		errField = ""
		reqType  = "empty"
		reqName  = "empty"
	)

	err := w.ResponseWriter.WriteMsg(res)
	if err != nil {
		errField = err.Error()
	}

	if len(res.Question) > 0 && res.Question[0].Qtype != dns.TypeNone {
		q := res.Question[0]

		reqType = dns.TypeToString[q.Qtype]
		reqName = q.Name
	}

	w.logger.Printf(
		"DNS %d %s %s %s %s %d '%s'",
		time.Since(w.start),
		w.RemoteAddr().String(),
		reqType,
		reqName,
		dns.RcodeToString[res.Rcode],
		len(res.Answer),
		errField,
	)

	return err
}

type loggingStore struct {
	logger *log.Logger
	next   store
}

func newLoggingStore(logger *log.Logger, next store) store {
	return &loggingStore{
		logger: logger,
		next:   next,
	}
}

func (s *loggingStore) getInstances(i info) (is instances, err error) {
	defer func(start time.Time) {
		s.log(time.Since(start), "getInstances", err, i.addr())
	}(time.Now())

	return s.next.getInstances(i)
}

func (s *loggingStore) getServers(zone string) (is instances, err error) {
	defer func(start time.Time) {
		s.log(time.Since(start), "getServers", err, zone)
	}(time.Now())

	return s.next.getServers(zone)
}

func (s *loggingStore) log(took time.Duration, op string, err error, input string) {
	if err == nil {
		return
	}

	label := errToLabel[errUntracked]

	switch e := err.(type) {
	case *glimpseError:
		label = errToLabel[e.err]
	}

	s.logger.Printf("STORE %d %s %s %s", took, op, label, input)
}
