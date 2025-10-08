package promauto

import "aggregator-service/app/src/infra/prometheus"

func NewCounter(opts prometheus.CounterOpts) *prometheus.Counter {
	counter := prometheus.NewCounter(opts)
	prometheus.MustRegister(counter)
	return counter
}

func NewHistogram(opts prometheus.HistogramOpts) *prometheus.Histogram {
	histogram := prometheus.NewHistogram(opts)
	prometheus.MustRegister(histogram)
	return histogram
}

func NewGauge(opts prometheus.GaugeOpts) *prometheus.Gauge {
	gauge := prometheus.NewGauge(opts)
	prometheus.MustRegister(gauge)
	return gauge
}
