package promauto

import (
	"bytes"
	"testing"

	"aggregator-service/app/src/infra/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestNewCounterRegistersMetric(t *testing.T) {
	t.Log("Шаг 1: создаём счётчик и проверяем его регистрацию")
	counter := NewCounter(prometheus.CounterOpts{Name: "promauto_counter_test"})
	counter.Inc()

	var buf bytes.Buffer
	prometheus.Gather(&buf)
	assert.Contains(t, buf.String(), "promauto_counter_test 1")
}

func TestNewHistogramRegistersMetric(t *testing.T) {
	t.Log("Шаг 1: создаём гистограмму и наблюдаем значение")
	histogram := NewHistogram(prometheus.HistogramOpts{Name: "promauto_hist_test", Buckets: []float64{1}})
	histogram.Observe(1)

	var buf bytes.Buffer
	prometheus.Gather(&buf)
	assert.Contains(t, buf.String(), "promauto_hist_test_bucket{le=\"1\"} 1")
}

func TestNewGaugeRegistersMetric(t *testing.T) {
	t.Log("Шаг 1: создаём gauge и устанавливаем значение")
	gauge := NewGauge(prometheus.GaugeOpts{Name: "promauto_gauge_test"})
	gauge.Set(5)

	var buf bytes.Buffer
	prometheus.Gather(&buf)
	assert.Contains(t, buf.String(), "promauto_gauge_test 5")
}
