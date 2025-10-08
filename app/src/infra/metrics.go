package infra

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// HTTP metrics
	HttpRequestsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	})
	HttpRequestErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "http_request_errors_total",
		Help: "Total number of HTTP request errors",
	})
	ProcessingDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "aggregator_processing_duration_seconds",
		Help:    "Duration of request processing in seconds",
		Buckets: prometheus.DefBuckets,
	})

	// Database metrics
	DbBatchFlushTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aggregator_db_batch_flush_total",
		Help: "Total number of database batch flush operations",
	})
	DbBatchDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "aggregator_db_batch_duration_seconds",
		Help:    "Duration of database batch flush operations in seconds",
		Buckets: prometheus.DefBuckets,
	})
	DbBatchSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "aggregator_db_batch_size",
		Help: "Size of the last flushed batch",
	})
	DbBatchWaitSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "aggregator_db_batch_wait_seconds",
		Help:    "Wait time before batch flush (seconds)",
		Buckets: prometheus.DefBuckets,
	})

	// Generator metrics
	PacketsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aggregator_packets_total",
		Help: "Total number of packets produced by the generator",
	})

	// Worker pool metrics
	WorkerPoolActiveGoroutines = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "aggregator_worker_pool_active_goroutines",
		Help: "Number of active worker pool goroutines",
	})

	registerOnce      sync.Once
	metricsServerOnce sync.Once
)

func init() {
	InitMetrics()
}

// InitMetrics registers all Prometheus collectors used by the application.
func InitMetrics() {
	registerOnce.Do(func() {
		prometheus.MustRegister(
			HttpRequestsTotal,
			HttpRequestErrorsTotal,
			ProcessingDurationSeconds,
			DbBatchFlushTotal,
			DbBatchDurationSeconds,
			DbBatchSize,
			DbBatchWaitSeconds,
			PacketsTotal,
			WorkerPoolActiveGoroutines,
		)
	})
}

// Handler returns an HTTP handler that exposes the registered Prometheus metrics.
func Handler() http.Handler {
	InitMetrics()
	return promhttp.Handler()
}

// StartMetricsServer exposes Prometheus metrics on :2112/metrics using promhttp.
func StartMetricsServer(logger *Logger) {
	InitMetrics()
	metricsServerOnce.Do(func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())

		go func() {
			if err := http.ListenAndServe(":2112", mux); err != nil {
				if logger != nil {
					logger.Printf(context.Background(), "metrics server error: %v", err)
				}
			}
		}()
	})
}

// HTTPMiddleware instruments HTTP handlers with request/latency metrics.
func HTTPMiddleware(pathResolver func(*http.Request) string) func(http.Handler) http.Handler {
	InitMetrics()
	if pathResolver == nil {
		pathResolver = func(r *http.Request) string {
			if r == nil {
				return "unknown"
			}
			return r.URL.Path
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r == nil {
				HttpRequestErrorsTotal.Inc()
				http.Error(w, "invalid request", http.StatusBadRequest)
				return
			}

			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()

			defer func() {
				duration := time.Since(start)
				ProcessingDurationSeconds.Observe(duration.Seconds())
				HttpRequestsTotal.Inc()

				if recorder.Status() >= http.StatusBadRequest {
					HttpRequestErrorsTotal.Inc()
				}
			}()

			next.ServeHTTP(recorder, r)
		})
	}
}

// GRPCUnaryInterceptor instruments gRPC unary handlers with request/latency metrics.
func GRPCUnaryInterceptor() grpc.UnaryServerInterceptor {
	InitMetrics()
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		start := time.Now()

		defer func() {
			duration := time.Since(start)
			ProcessingDurationSeconds.Observe(duration.Seconds())
			HttpRequestsTotal.Inc()

			if status.Code(err) != codes.OK {
				HttpRequestErrorsTotal.Inc()
			}
		}()

		return handler(ctx, req)
	}
}

// RecordDBBatchFlush tracks a completed database batch flush.
func RecordDBBatchFlush(duration time.Duration) {
	InitMetrics()
	if duration < 0 {
		duration = 0
	}
	DbBatchFlushTotal.Inc()
	DbBatchDurationSeconds.Observe(duration.Seconds())
}

// IncGeneratorPackets increments the generator packet counter.
func IncGeneratorPackets() {
	InitMetrics()
	PacketsTotal.Inc()
}

// WorkerStarted increments the worker pool active goroutines gauge.
func WorkerStarted() {
	InitMetrics()
	WorkerPoolActiveGoroutines.Inc()
}

// WorkerFinished decrements the worker pool active goroutines gauge.
func WorkerFinished() {
	InitMetrics()
	WorkerPoolActiveGoroutines.Dec()
}

// statusRecorder captures the response status code for instrumentation.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Status() int {
	return r.status
}
