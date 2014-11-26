package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
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

// consulCollector implements the prometheus.Collector interface.
type consulCollector struct {
	bin     string
	errc    chan error
	metrics map[string]prometheus.Gauge
}

func newConsulCollector(bin string, errc chan error) prometheus.Collector {
	return &consulCollector{
		bin:     bin,
		errc:    errc,
		metrics: map[string]prometheus.Gauge{},
	}
}

func (c *consulCollector) Collect(metricc chan<- prometheus.Metric) {
	err := c.updateMetrics()
	if err != nil {
		c.errc <- err
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
		// TODO(alx): prometheus.MustRegister will report an unrelated error if we
		//						just bubble up here, instead we panic. This needs to be
		//						addressed.
		panic(err)
	}

	for _, m := range c.metrics {
		descc <- m.Desc()
	}
}

func (c *consulCollector) updateMetrics() error {
	stats, err := getConsulStats(c.bin)
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

func getConsulStats(bin string) (consulStats, error) {
	info := exec.Command(bin, "info")

	outPipe, err := info.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := info.Start(); err != nil {
		return nil, err
	}

	stats, err := parseConsulStats(outPipe)
	if err != nil {
		return nil, err
	}

	if err := info.Wait(); err != nil {
		return nil, err
	}

	return stats, nil
}

func parseConsulStats(r io.Reader) (consulStats, error) {
	var (
		s     = bufio.NewScanner(r)
		stats = consulStats{}

		ignoredFields = map[string]struct{}{
			"arch":    struct{}{},
			"os":      struct{}{},
			"state":   struct{}{},
			"version": struct{}{},
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
