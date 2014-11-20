package main

import (
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

func dnsMetricsHandler(next dns.Handler) dns.HandlerFunc {
	var (
		durations = prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Namespace: "glimpse_agent",
				Subsystem: "dns",
				Name:      "request_duration_microseconds",
				Help:      "DNS request latencies in microseconds.",
			},
			[]string{"protocol", "qtype", "rcode"},
		)
	)

	prometheus.MustRegister(durations)

	return func(w dns.ResponseWriter, req *dns.Msg) {
		var (
			prot  = "unknown"
			qtype = "unknown"
			rcode = "unknown"

			b = &bufferedWriter{w, nil}
			s = time.Now()
		)

		if _, ok := w.RemoteAddr().(*net.UDPAddr); ok {
			prot = "udp"
		} else if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
			prot = "tcp"
		}

		if len(req.Question) == 1 {
			qtype = strings.ToLower(dns.TypeToString[req.Question[0].Qtype])
		}

		next.ServeDNS(b, req)

		if b.msg != nil {
			w.WriteMsg(b.msg)
			rcode = strings.ToLower(dns.RcodeToString[b.msg.Rcode])
		}

		duration := float64(time.Since(s)) / float64(time.Microsecond)
		durations.WithLabelValues(prot, qtype, rcode).Observe(duration)
	}
}
