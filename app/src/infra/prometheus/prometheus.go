package prometheus

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

type CounterOpts struct {
	Namespace string
	Subsystem string
	Name      string
	Help      string
}

type HistogramOpts struct {
	Namespace string
	Subsystem string
	Name      string
	Help      string
	Buckets   []float64
}

type GaugeOpts struct {
	Namespace string
	Subsystem string
	Name      string
	Help      string
}

var DefBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type descriptor struct {
	name string
	help string
}

type metric interface {
	descriptor() descriptor
	write(io.Writer)
}

var (
	registryMu sync.RWMutex
	registry   = make(map[string]metric)
)

func MustRegister(metrics ...metric) {
	registryMu.Lock()
	defer registryMu.Unlock()

	for _, m := range metrics {
		desc := m.descriptor()
		if _, exists := registry[desc.name]; exists {
			panic(fmt.Sprintf("prometheus: duplicate metric %s", desc.name))
		}
		registry[desc.name] = m
	}
}

func Gather(w io.Writer) {
	registryMu.RLock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	metrics := make([]metric, 0, len(names))
	for _, name := range names {
		metrics = append(metrics, registry[name])
	}
	registryMu.RUnlock()

	for _, m := range metrics {
		m.write(w)
	}
}

type Counter struct {
	desc  descriptor
	mu    sync.Mutex
	value float64
}

func NewCounter(opts CounterOpts) *Counter {
	return &Counter{desc: descriptor{name: buildFQName(opts.Namespace, opts.Subsystem, opts.Name), help: opts.Help}}
}

func (c *Counter) Inc() {
	c.Add(1)
}

func (c *Counter) Add(v float64) {
	if v < 0 {
		return
	}
	c.mu.Lock()
	c.value += v
	c.mu.Unlock()
}

func (c *Counter) Value() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

func (c *Counter) descriptor() descriptor {
	return c.desc
}

func (c *Counter) write(w io.Writer) {
	c.mu.Lock()
	value := c.value
	c.mu.Unlock()

	fmt.Fprintf(w, "# HELP %s %s\n", c.desc.name, escapeHelp(c.desc.help))
	fmt.Fprintf(w, "# TYPE %s counter\n", c.desc.name)
	fmt.Fprintf(w, "%s %v\n", c.desc.name, value)
}

type Histogram struct {
	desc    descriptor
	buckets []float64
	counts  []uint64
	mu      sync.Mutex
	sum     float64
	count   uint64
}

func NewHistogram(opts HistogramOpts) *Histogram {
	bounds := append([]float64(nil), opts.Buckets...)
	if len(bounds) == 0 {
		bounds = append([]float64(nil), DefBuckets...)
	}
	sort.Float64s(bounds)
	counts := make([]uint64, len(bounds))
	return &Histogram{
		desc:    descriptor{name: buildFQName(opts.Namespace, opts.Subsystem, opts.Name), help: opts.Help},
		buckets: bounds,
		counts:  counts,
	}
}

func (h *Histogram) Observe(value float64) {
	if value < 0 {
		value = 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sum += value
	h.count++
	for i, upper := range h.buckets {
		if value <= upper {
			h.counts[i]++
		}
	}
}

func (h *Histogram) descriptor() descriptor {
	return h.desc
}

func (h *Histogram) write(w io.Writer) {
	h.mu.Lock()
	sum := h.sum
	count := h.count
	counts := append([]uint64(nil), h.counts...)
	buckets := append([]float64(nil), h.buckets...)
	h.mu.Unlock()

	fmt.Fprintf(w, "# HELP %s %s\n", h.desc.name, escapeHelp(h.desc.help))
	fmt.Fprintf(w, "# TYPE %s histogram\n", h.desc.name)

	for i, upper := range buckets {
		fmt.Fprintf(w, "%s_bucket{le=\"%g\"} %d\n", h.desc.name, upper, counts[i])
	}
	fmt.Fprintf(w, "%s_bucket{le=\"+Inf\"} %d\n", h.desc.name, count)
	fmt.Fprintf(w, "%s_sum %g\n", h.desc.name, sum)
	fmt.Fprintf(w, "%s_count %d\n", h.desc.name, count)
}

type Gauge struct {
	desc  descriptor
	mu    sync.Mutex
	value float64
}

func NewGauge(opts GaugeOpts) *Gauge {
	return &Gauge{desc: descriptor{name: buildFQName(opts.Namespace, opts.Subsystem, opts.Name), help: opts.Help}}
}

func (g *Gauge) Set(value float64) {
	g.mu.Lock()
	g.value = value
	g.mu.Unlock()
}

func (g *Gauge) Add(delta float64) {
	g.mu.Lock()
	g.value += delta
	g.mu.Unlock()
}

func (g *Gauge) Value() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.value
}

func (g *Gauge) Inc() {
	g.Add(1)
}

func (g *Gauge) Dec() {
	g.Add(-1)
}

func (g *Gauge) descriptor() descriptor {
	return g.desc
}

func (g *Gauge) write(w io.Writer) {
	g.mu.Lock()
	value := g.value
	g.mu.Unlock()

	fmt.Fprintf(w, "# HELP %s %s\n", g.desc.name, escapeHelp(g.desc.help))
	fmt.Fprintf(w, "# TYPE %s gauge\n", g.desc.name)
	fmt.Fprintf(w, "%s %v\n", g.desc.name, value)
}

func escapeHelp(input string) string {
	replacer := strings.NewReplacer("\n", "\\n")
	return replacer.Replace(input)
}

func buildFQName(namespace, subsystem, name string) string {
	var parts []string
	for _, value := range []string{namespace, subsystem, name} {
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "_")
}
