package prometheus

import "github.com/prometheus/procfs"

type processCollector struct {
	pid             int
	collectFn       func(chan<- Metric)
	pidFn           func() int
	cpuTotal        Counter
	openFDs, maxFDs Gauge
	vsize, rss      Gauge
	startTime       Gauge
}

// NewProcessCollector returns a collector which exports the current state of
// process metrics including cpu, memory and file descriptor usage as well as
// the process start time for the given process id under the given namespace.
func NewProcessCollector(pid int, namespace string) *processCollector {
	return NewProcessCollectorPIDFn(func() int { return pid }, namespace)
}

// NewProcessCollectorPIDFn returns a collector which exports the current state
// of process metrics including cpu, memory and file descriptor usage as well
// as the process start time under the given namespace. The given pidFn is
// called on each collect and is used to determine the process to export
// metrics for.
func NewProcessCollectorPIDFn(
	pidFn func() int,
	namespace string,
) *processCollector {
	c := processCollector{
		pidFn:     pidFn,
		collectFn: noopCollect,

		cpuTotal: NewCounter(CounterOpts{
			Namespace: namespace,
			Name:      "process_cpu_seconds_total",
			Help:      "Total user and system CPU time spent in seconds.",
		}),
		openFDs: NewGauge(GaugeOpts{
			Namespace: namespace,
			Name:      "process_open_fds",
			Help:      "Number of open file descriptors.",
		}),
		maxFDs: NewGauge(GaugeOpts{
			Namespace: namespace,
			Name:      "process_max_fds",
			Help:      "Maximum number of open file descriptors.",
		}),
		vsize: NewGauge(GaugeOpts{
			Namespace: namespace,
			Name:      "process_virtual_memory_bytes",
			Help:      "Virtual memory size in bytes.",
		}),
		rss: NewGauge(GaugeOpts{
			Namespace: namespace,
			Name:      "process_resident_memory_bytes",
			Help:      "Resident memory size in bytes.",
		}),
		startTime: NewGauge(GaugeOpts{
			Namespace: namespace,
			Name:      "process_start_time_seconds",
			Help:      "Start time of the process since unix epoch in seconds.",
		}),
	}

	// Use procfs to export metrics if available.
	if _, err := procfs.NewStat(); err == nil {
		c.collectFn = c.procfsCollect
	}

	return &c
}

// Describe returns all descriptions of the collector.
func (c *processCollector) Describe(ch chan<- *Desc) {
	ch <- c.cpuTotal.Desc()
	ch <- c.openFDs.Desc()
	ch <- c.maxFDs.Desc()
	ch <- c.vsize.Desc()
	ch <- c.rss.Desc()
	ch <- c.startTime.Desc()
}

// Collect returns the current state of all metrics of the collector.
func (c *processCollector) Collect(ch chan<- Metric) {
	c.collectFn(ch)
}

func noopCollect(ch chan<- Metric) {}

func (c *processCollector) procfsCollect(ch chan<- Metric) {
	p, err := procfs.NewProc(c.pidFn())
	if err != nil {
		return
	}

	if stat, err := p.NewStat(); err == nil {
		c.cpuTotal.Set(stat.CPUTime())
		ch <- c.cpuTotal
		c.vsize.Set(float64(stat.VirtualMemory()))
		ch <- c.vsize
		c.rss.Set(float64(stat.ResidentMemory()))
		ch <- c.rss

		if startTime, err := stat.StartTime(); err == nil {
			c.startTime.Set(startTime)
			ch <- c.startTime
		}
	}

	if fds, err := p.FileDescriptorsLen(); err == nil {
		c.openFDs.Set(float64(fds))
		ch <- c.openFDs
	}

	if limits, err := p.NewLimits(); err == nil {
		c.maxFDs.Set(float64(limits.OpenFiles))
		ch <- c.maxFDs
	}
}
