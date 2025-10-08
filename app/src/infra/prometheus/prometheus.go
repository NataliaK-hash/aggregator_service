package prometheus

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// CounterOpts configures a Counter metric.
type CounterOpts struct {
	Namespace string
	Subsystem string
	Name      string
	Help      string
}

// HistogramOpts configures a Histogram metric.
type HistogramOpts struct {
	Namespace string
	Subsystem string
	Name      string
	Help      string
	Buckets   []float64
}

// DefBuckets mirrors the default Prometheus histogram buckets.
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

// MustRegister registers provided metrics or panics if registration fails.
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

// Counter implements an always-increasing numeric value.
type Counter struct {
	desc  descriptor
	mu    sync.Mutex
	value float64
}

// NewCounter creates a counter using provided options.
func NewCounter(opts CounterOpts) *Counter {
	return &Counter{desc: descriptor{name: buildFQName(opts.Namespace, opts.Subsystem, opts.Name), help: opts.Help}}
}

// Inc increases the counter value by one.
func (c *Counter) Inc() {
	c.mu.Lock()
	c.value++
	c.mu.Unlock()
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

// Histogram records observations into configurable buckets.
type Histogram struct {
	desc    descriptor
	buckets []float64
	counts  []uint64
	mu      sync.Mutex
	sum     float64
	count   uint64
}

// NewHistogram constructs a histogram metric.
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

// Observe records a single observation into the histogram.
func (h *Histogram) Observe(value float64) {
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

	var cumulative uint64
	for i, upper := range buckets {
		cumulative += counts[i]
		fmt.Fprintf(w, "%s_bucket{le=\"%g\"} %d\n", h.desc.name, upper, cumulative)
	}
	fmt.Fprintf(w, "%s_bucket{le=\"+Inf\"} %d\n", h.desc.name, count)
	fmt.Fprintf(w, "%s_sum %g\n", h.desc.name, sum)
	fmt.Fprintf(w, "%s_count %d\n", h.desc.name, count)
}

func escapeHelp(input string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "\n", "\\n")
	return replacer.Replace(input)
}

func buildFQName(namespace, subsystem, name string) string {
	var parts []string
	for _, v := range []string{namespace, subsystem, name} {
		if v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, "_")
}

// CounterVec groups counters that differ only by their label values.
type CounterVec struct {
	desc       descriptor
	labelNames []string

	mu      sync.Mutex
	entries map[string]*counterVecEntry
	metrics map[string]*CounterVecCounter
}

type counterVecEntry struct {
	labels []string
	value  float64
}

// CounterVecCounter represents a single counter instance within a CounterVec.
type CounterVecCounter struct {
	vec *CounterVec
	key string
}

// NewCounterVec constructs a vector of counters sharing the provided name and help text.
func NewCounterVec(opts CounterOpts, labelNames []string) *CounterVec {
	return &CounterVec{
		desc:       descriptor{name: buildFQName(opts.Namespace, opts.Subsystem, opts.Name), help: opts.Help},
		labelNames: append([]string(nil), labelNames...),
		entries:    make(map[string]*counterVecEntry),
		metrics:    make(map[string]*CounterVecCounter),
	}
}

// WithLabelValues retrieves or creates a counter for the provided label values.
func (c *CounterVec) WithLabelValues(labelValues ...string) *CounterVecCounter {
	if len(labelValues) != len(c.labelNames) {
		panic(fmt.Sprintf("prometheus: expected %d labels, got %d", len(c.labelNames), len(labelValues)))
	}
	key := joinLabelValues(labelValues)

	c.mu.Lock()
	defer c.mu.Unlock()

	if metric, ok := c.metrics[key]; ok {
		return metric
	}

	entry := &counterVecEntry{labels: append([]string(nil), labelValues...)}
	c.entries[key] = entry
	metric := &CounterVecCounter{vec: c, key: key}
	c.metrics[key] = metric
	return metric
}

// Inc increments the counter by one.
func (c *CounterVecCounter) Inc() {
	c.Add(1)
}

// Add increases the counter by the provided value.
func (c *CounterVecCounter) Add(v float64) {
	if v < 0 {
		panic("prometheus: counter cannot decrease")
	}
	c.vec.mu.Lock()
	c.vec.entries[c.key].value += v
	c.vec.mu.Unlock()
}

func (c *CounterVec) descriptor() descriptor {
	return c.desc
}

func (c *CounterVec) write(w io.Writer) {
	fmt.Fprintf(w, "# HELP %s %s\n", c.desc.name, escapeHelp(c.desc.help))
	fmt.Fprintf(w, "# TYPE %s counter\n", c.desc.name)

	c.mu.Lock()
	entries := make([]counterVecEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		entries = append(entries, counterVecEntry{
			labels: append([]string(nil), entry.labels...),
			value:  entry.value,
		})
	}
	c.mu.Unlock()

	sort.Slice(entries, func(i, j int) bool {
		return compareLabels(entries[i].labels, entries[j].labels) < 0
	})

	for _, entry := range entries {
		fmt.Fprintf(w, "%s{%s} %v\n", c.desc.name, formatLabels(c.labelNames, entry.labels), entry.value)
	}
}

// HistogramVec groups histograms that share the same name but differ by label values.
type HistogramVec struct {
	desc       descriptor
	labelNames []string
	buckets    []float64

	mu      sync.Mutex
	entries map[string]*histogramVecEntry
	metrics map[string]*HistogramVecObserver
}

type histogramVecEntry struct {
	labels  []string
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
}

// HistogramVecObserver represents a histogram instance inside HistogramVec.
type HistogramVecObserver struct {
	vec *HistogramVec
	key string
}

// NewHistogramVec constructs a labelled histogram vector using provided options.
func NewHistogramVec(opts HistogramOpts, labelNames []string) *HistogramVec {
	bounds := append([]float64(nil), opts.Buckets...)
	if len(bounds) == 0 {
		bounds = append([]float64(nil), DefBuckets...)
	}
	sort.Float64s(bounds)

	return &HistogramVec{
		desc:       descriptor{name: buildFQName(opts.Namespace, opts.Subsystem, opts.Name), help: opts.Help},
		labelNames: append([]string(nil), labelNames...),
		buckets:    bounds,
		entries:    make(map[string]*histogramVecEntry),
		metrics:    make(map[string]*HistogramVecObserver),
	}
}

// WithLabelValues retrieves or creates a histogram for the provided label values.
func (h *HistogramVec) WithLabelValues(labelValues ...string) *HistogramVecObserver {
	if len(labelValues) != len(h.labelNames) {
		panic(fmt.Sprintf("prometheus: expected %d labels, got %d", len(h.labelNames), len(labelValues)))
	}
	key := joinLabelValues(labelValues)

	h.mu.Lock()
	defer h.mu.Unlock()

	if observer, ok := h.metrics[key]; ok {
		return observer
	}

	entry := &histogramVecEntry{
		labels:  append([]string(nil), labelValues...),
		buckets: append([]float64(nil), h.buckets...),
		counts:  make([]uint64, len(h.buckets)),
	}
	h.entries[key] = entry
	observer := &HistogramVecObserver{vec: h, key: key}
	h.metrics[key] = observer
	return observer
}

// Observe records a new sample for the histogram.
func (h *HistogramVecObserver) Observe(value float64) {
	h.vec.mu.Lock()
	entry := h.vec.entries[h.key]
	entry.sum += value
	entry.count++
	for i, upper := range entry.buckets {
		if value <= upper {
			entry.counts[i]++
		}
	}
	h.vec.mu.Unlock()
}

func (h *HistogramVec) descriptor() descriptor {
	return h.desc
}

func (h *HistogramVec) write(w io.Writer) {
	fmt.Fprintf(w, "# HELP %s %s\n", h.desc.name, escapeHelp(h.desc.help))
	fmt.Fprintf(w, "# TYPE %s histogram\n", h.desc.name)

	h.mu.Lock()
	entries := make([]histogramVecEntry, 0, len(h.entries))
	for _, entry := range h.entries {
		entries = append(entries, histogramVecEntry{
			labels:  append([]string(nil), entry.labels...),
			buckets: append([]float64(nil), entry.buckets...),
			counts:  append([]uint64(nil), entry.counts...),
			sum:     entry.sum,
			count:   entry.count,
		})
	}
	h.mu.Unlock()

	sort.Slice(entries, func(i, j int) bool {
		return compareLabels(entries[i].labels, entries[j].labels) < 0
	})

	for _, entry := range entries {
		baseLabels := formatLabels(h.labelNames, entry.labels)
		cumulative := uint64(0)
		for i, upper := range entry.buckets {
			cumulative += entry.counts[i]
			fmt.Fprintf(w, "%s_bucket{%s,le=\"%g\"} %d\n", h.desc.name, baseLabels, upper, cumulative)
		}
		fmt.Fprintf(w, "%s_bucket{%s,le=\"+Inf\"} %d\n", h.desc.name, baseLabels, entry.count)
		fmt.Fprintf(w, "%s_sum{%s} %g\n", h.desc.name, baseLabels, entry.sum)
		fmt.Fprintf(w, "%s_count{%s} %d\n", h.desc.name, baseLabels, entry.count)
	}
}

func joinLabelValues(values []string) string {
	return strings.Join(values, "\xff")
}

func compareLabels(a, b []string) int {
	max := len(a)
	if len(b) > max {
		max = len(b)
	}
	for i := 0; i < max; i++ {
		var av, bv string
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av == bv {
			continue
		}
		if av < bv {
			return -1
		}
		return 1
	}
	return len(a) - len(b)
}

func formatLabels(names, values []string) string {
	parts := make([]string, len(names))
	for i, name := range names {
		value := ""
		if i < len(values) {
			value = values[i]
		}
		parts[i] = fmt.Sprintf("%s=\"%s\"", name, escapeLabelValue(value))
	}
	return strings.Join(parts, ",")
}

func escapeLabelValue(input string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\"", "\\\"")
	return replacer.Replace(input)
}

// WriteTo renders all registered metrics into the provided writer.
func WriteTo(w io.Writer) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		registry[name].write(w)
	}
}
