package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace   = "glimpse_agent"
	consulAgent = "consul agent"
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
		[]string{"operation"},
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
	prometheus.MustRegister(
		prometheus.NewProcessCollectorPIDFn(consulAgentPid, "consul"))
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
			qtype = dns.TypeToString[req.Question[0].Qtype]
		}

		next.ServeDNS(buffer, req)

		if buffer.msg != nil {
			w.WriteMsg(buffer.msg)
			rcode = dns.RcodeToString[buffer.msg.Rcode]
		}

		duration := float64(time.Since(start)) / float64(time.Microsecond)
		dnsDurations.WithLabelValues(prot, qtype, rcode).Observe(duration)
	}
}

// consulCollector implements the prometheus.Collector interface.
type consulCollector struct {
	info    string
	metrics map[string]prometheus.Gauge
}

func newConsulCollector(info string) prometheus.Collector {
	return &consulCollector{
		info:    info,
		metrics: map[string]prometheus.Gauge{},
	}
}

func (c *consulCollector) Collect(metricc chan<- prometheus.Metric) {
	err := c.updateMetrics()
	if err != nil {
		return
	}

	for _, m := range c.metrics {
		m.Collect(metricc)
	}
}

// TODO(alx): Clarify proper usage of Describe and improve error handling.
func (c *consulCollector) Describe(descc chan<- *prometheus.Desc) {
	err := c.updateMetrics()
	if err != nil {
		return
	}

	for _, m := range c.metrics {
		descc <- m.Desc()
	}
}

func (c *consulCollector) updateMetrics() error {
	stats, err := getConsulStats(c.info)
	if err != nil {
		return fmt.Errorf("consul info failed: %s", err)
	}

	for name, value := range stats {
		if _, ok := c.metrics[name]; !ok {
			c.metrics[name] = prometheus.NewGauge(
				prometheus.GaugeOpts{
					Namespace: "consul",
					Name:      name,
					Help:      fmt.Sprintf("%s from consul info", name),
				},
			)
		}

		c.metrics[name].Set(float64(value))
	}

	return nil
}

type consulStats map[string]int64

type metricsStore struct {
	next store
}

func newMetricsStore(next store) *metricsStore {
	return &metricsStore{next: next}
}

func (s *metricsStore) getInstances(i info) (instances, error) {
	var (
		op     = "getInstances"
		labels = prometheus.Labels{
			"service":   i.service,
			"job":       i.job,
			"env":       i.env,
			"product":   i.product,
			"zone":      i.zone,
			"operation": op,
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
	storeDurations.WithLabelValues(op).Observe(duration)
	storeCounts.With(labels).Set(float64(len(ins)))

	return ins, err
}

func (s *metricsStore) getServers(zone string) (instances, error) {
	var (
		op    = "getServers"
		start = time.Now()
	)

	r, err := s.next.getServers(zone)
	duration := float64(time.Since(start)) / float64(time.Microsecond)
	storeDurations.WithLabelValues(op).Observe(duration)

	return r, err
}

func getConsulStats(info string) (consulStats, error) {
	cmd := strings.Split(info, " ")
	output, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		return nil, err
	}

	stats, err := parseConsulStats(output)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func parseConsulStats(b []byte) (consulStats, error) {
	var (
		s     = bufio.NewScanner(bytes.NewBuffer(b))
		stats = consulStats{}

		ignoredFields = map[string]struct{}{
			"arch":         struct{}{},
			"os":           struct{}{},
			"state":        struct{}{},
			"version":      struct{}{},
			"last_contact": struct{}{},
		}
		ignoredKeys = map[string]struct{}{
			"build": struct{}{},
		}

		key string
	)

	for s.Scan() {
		line := strings.TrimSpace(s.Text())

		if strings.Contains(line, ":") {
			key = strings.TrimSuffix(line, ":")
		}

		if _, ok := ignoredKeys[key]; ok {
			continue
		}

		if strings.Contains(line, "=") {
			var (
				parts        = strings.SplitN(line, "=", 2)
				field, value = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
				name         = strings.Join([]string{key, field}, "_")
			)

			if _, ok := ignoredFields[field]; ok {
				continue
			}

			switch value {
			case "true":
				stats[name] = 1
			case "false":
				stats[name] = 0
			case "never":
				stats[name] = 0
			default:
				i, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					return nil, err
				}
				stats[name] = i
			}
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

func consulAgentPid() (int, error) {
	out, err := exec.Command("pgrep", "-f", consulAgent).Output()
	if err != nil {
		return 0, fmt.Errorf("could not get pid of %s: %s", consulAgent, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("could not parse pid of %s: %s", consulAgent, err)
	}
	return pid, nil
}
