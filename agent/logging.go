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
		qType    = "empty"
		qName    = "empty"
	)

	err := w.ResponseWriter.WriteMsg(res)
	if err != nil {
		errField = " error: " + err.Error()
	}

	if len(res.Question) > 0 && res.Question[0].Qtype != dns.TypeNone {
		q := res.Question[0]

		qType = dns.TypeToString[q.Qtype]
		qName = q.Name
	}

	w.logger.Printf(
		"DNS %dms %s %s %s %s %d%s",
		time.Since(w.start)/time.Millisecond,
		w.RemoteAddr().String(),
		qType,
		qName,
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
		s.log(time.Since(start), "getInstances", i.addr(), err)
	}(time.Now())

	return s.next.getInstances(i)
}

func (s *loggingStore) getServers(zone string) (is instances, err error) {
	defer func(start time.Time) {
		s.log(time.Since(start), "getServers", zone, err)
	}(time.Now())

	return s.next.getServers(zone)
}

func (s *loggingStore) log(took time.Duration, op, input string, err error) {
	if err == nil {
		return
	}

	s.logger.Printf("STORE %dms %s %s error: %s", took/time.Millisecond, op, input, err)
}
