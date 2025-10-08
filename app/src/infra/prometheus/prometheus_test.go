package prometheus

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func resetRegistry() {
	registryMu.Lock()
	registry = make(map[string]metric)
	registryMu.Unlock()
}

func TestMustRegisterPreventsDuplicates(t *testing.T) {
	t.Log("Шаг 1: регистрируем метрику и проверяем защиту от дубликатов")
	resetRegistry()
	counter := NewCounter(CounterOpts{Name: "requests_total", Help: "total"})
	MustRegister(counter)

	assert.Panics(t, func() { MustRegister(counter) })
}

func TestGatherWritesMetrics(t *testing.T) {
	t.Log("Шаг 1: добавляем метрику и собираем вывод")
	resetRegistry()
	counter := NewCounter(CounterOpts{Name: "hits_total", Help: "hits"})
	counter.Inc()
	MustRegister(counter)

	var buf bytes.Buffer
	Gather(&buf)

	output := buf.String()
	t.Log("Шаг 2: проверяем, что вывод содержит значения")
	assert.Contains(t, output, "# HELP hits_total hits")
	assert.Contains(t, output, "hits_total 1")
}

func TestCounterOperations(t *testing.T) {
	t.Log("Шаг 1: выполняем операции инкремента и добавления")
	counter := NewCounter(CounterOpts{Name: "events_total"})
	counter.Add(-1)
	counter.Inc()
	counter.Add(2)

	var buf bytes.Buffer
	counter.write(&buf)

	assert.Contains(t, buf.String(), "events_total 3")
}

func TestHistogramObserve(t *testing.T) {
	t.Log("Шаг 1: наблюдаем значения для гистограммы")
	histogram := NewHistogram(HistogramOpts{Name: "latency_seconds", Buckets: []float64{1, 2}})
	histogram.Observe(-1)
	histogram.Observe(1.5)

	var buf bytes.Buffer
	histogram.write(&buf)

	output := buf.String()
	t.Log("Шаг 2: проверяем заполненные бакеты")
	assert.Contains(t, output, "latency_seconds_bucket{le=\"1\"} 1")
	assert.Contains(t, output, "latency_seconds_bucket{le=\"2\"} 2")
	assert.Contains(t, output, "latency_seconds_sum")
}

func TestGaugeOperations(t *testing.T) {
	t.Log("Шаг 1: изменяем значение gauge и проверяем результат")
	gauge := NewGauge(GaugeOpts{Name: "inflight"})
	gauge.Set(2)
	gauge.Add(-1)
	gauge.Inc()
	gauge.Dec()

	var buf bytes.Buffer
	gauge.write(&buf)

	assert.Contains(t, buf.String(), "inflight 1")
}

func TestEscapeHelp(t *testing.T) {
	t.Log("Шаг 1: экранируем текст подсказки")
	result := escapeHelp("line\\nbreak")
	assert.Equal(t, "line\\nbreak", result)
}

func TestBuildFQName(t *testing.T) {
	t.Log("Шаг 1: собираем полное имя метрики")
	fq := buildFQName("ns", "sub", "name")
	assert.Equal(t, "ns_sub_name", fq)

	fq = buildFQName("", "sub", "name")
	assert.Equal(t, "sub_name", fq)
}

func TestRegisterOrderIsStable(t *testing.T) {
	t.Log("Шаг 1: регистрируем метрики и проверяем порядок")
	resetRegistry()
	first := NewCounter(CounterOpts{Name: "a"})
	second := NewCounter(CounterOpts{Name: "b"})
	MustRegister(second, first)

	var buf bytes.Buffer
	Gather(&buf)

	output := buf.String()
	assert.Less(t, strings.Index(output, "# HELP a"), strings.Index(output, "# HELP b"))
}
