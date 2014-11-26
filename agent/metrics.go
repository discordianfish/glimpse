package main

import (
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "glimpse_agent"
)

var (
	storeLabels = []string{
		"service",
		"job",
		"env",
		"product",
		"zone",
		"operation",
		"error",
	}

	dnsDurations = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: namespace,
			Subsystem: "dns",
			Name:      "request_duration_microseconds",
			Help:      "DNS request latencies in microseconds.",
		},
		[]string{"protocol", "qtype", "rcode"},
	)
	storeDurations = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: namespace,
			Subsystem: "consul",
			Name:      "request_duration_microseconds",
			Help:      "Consul API request latencies in microseconds.",
		},
		storeLabels,
	)
	storeCounts = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "consul",
			Name:      "responses",
			Help:      "Consul API responses.",
		},
		storeLabels,
	)
)

func init() {
	prometheus.MustRegister(dnsDurations)
	prometheus.MustRegister(storeDurations)
	prometheus.MustRegister(storeCounts)
}

func dnsMetricsHandler(next dns.Handler) dns.HandlerFunc {
	return func(w dns.ResponseWriter, req *dns.Msg) {
		var (
			prot  = "unknown"
			qtype = "unknown"
			rcode = "unknown"

			buffer = &bufferedWriter{w, nil}
			start  = time.Now()
		)

		if _, ok := w.RemoteAddr().(*net.UDPAddr); ok {
			prot = "udp"
		} else if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
			prot = "tcp"
		}

		if len(req.Question) == 1 {
			qtype = strings.ToLower(dns.TypeToString[req.Question[0].Qtype])
		}

		next.ServeDNS(buffer, req)

		if buffer.msg != nil {
			w.WriteMsg(buffer.msg)
			rcode = strings.ToLower(dns.RcodeToString[buffer.msg.Rcode])
		}

		duration := float64(time.Since(start)) / float64(time.Microsecond)
		dnsDurations.WithLabelValues(prot, qtype, rcode).Observe(duration)
	}
}

type metricsStore struct {
	next store
}

func newMetricsStore(next store) *metricsStore {
	return &metricsStore{next: next}
}

func (s *metricsStore) getInstances(i info) (instances, error) {
	var (
		labels = prometheus.Labels{
			"service":   i.service,
			"job":       i.job,
			"env":       i.env,
			"product":   i.product,
			"zone":      i.zone,
			"operation": "getInstances",
			"error":     "none",
		}
		start = time.Now()
	)

	ins, err := s.next.getInstances(i)
	if err != nil {
		switch e := err.(type) {
		case *glimpseError:
			labels["error"] = errToLabel[e.err]
		default:
			labels["error"] = errToLabel[errUntracked]
		}
	}

	duration := float64(time.Since(start)) / float64(time.Microsecond)
	storeDurations.With(labels).Observe(duration)
	storeCounts.With(labels).Set(float64(len(ins)))

	return ins, err
}
