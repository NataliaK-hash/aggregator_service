package infra

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"aggregator-service/app/src/infra/prometheus"
	"aggregator-service/app/src/infra/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	once sync.Once

	packetsCounter       *prometheus.Counter
	errorsCounter        *prometheus.Counter
	dbWritesCounter      *prometheus.Counter
	dbWriteErrorsCounter *prometheus.Counter
	dbWriteHist          *prometheus.Histogram
	dbBatchSizeHist      *prometheus.Histogram
	dbBatchWaitHist      *prometheus.Histogram
	httpRequests         *prometheus.CounterVec
	httpDuration         *prometheus.HistogramVec
	grpcRequests         *prometheus.CounterVec
	grpcDuration         *prometheus.HistogramVec
)

func initMetrics() {
	once.Do(func() {
		packetsCounter = prometheus.NewCounter(prometheus.CounterOpts{
			Name: "aggregator_packets_total",
			Help: "Total number of processed packets/units of work.",
		})

		errorsCounter = prometheus.NewCounter(prometheus.CounterOpts{
			Name: "aggregator_errors_total",
			Help: "Total number of processing errors (business + infrastructure).",
		})

		dbWritesCounter = prometheus.NewCounter(prometheus.CounterOpts{
			Name: "aggregator_db_writes_total",
			Help: "Successful database upserts.",
		})

		dbWriteErrorsCounter = prometheus.NewCounter(prometheus.CounterOpts{
			Name: "aggregator_db_write_errors_total",
			Help: "Database write errors.",
		})

		dbWriteHist = prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "aggregator_db_write_latency_seconds",
			Help:    "Latency of database writes in seconds.",
			Buckets: prometheus.DefBuckets,
		})

		dbBatchSizeHist = prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "aggregator_db_batch_size",
			Help:    "Number of measurements flushed per database batch.",
			Buckets: []float64{1, 2, 4, 8, 16, 32, 64, 128, 256, 512},
		})

		dbBatchWaitHist = prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "aggregator_db_batch_wait_seconds",
			Help:    "Time between the first measurement in a batch and the flush in seconds.",
			Buckets: prometheus.DefBuckets,
		})

		httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests processed by the aggregator service.",
		}, []string{"path", "method", "status"})

		httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"path", "method"})

		grpcRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests processed by the aggregator service.",
		}, []string{"method", "code"})

		grpcDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "gRPC request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method"})

		prometheus.MustRegister(
			packetsCounter,
			errorsCounter,
			dbWritesCounter,
			dbWriteErrorsCounter,
			dbWriteHist,
			dbBatchSizeHist,
			dbBatchWaitHist,
			httpRequests,
			httpDuration,
			grpcRequests,
			grpcDuration,
		)
	})
}

// Handler returns an HTTP handler that exposes the registered Prometheus metrics.
func Handler() http.Handler {
	initMetrics()
	return promhttp.Handler()
}

// IncPackets increments the total number of processed packets/units of work.
func IncPackets() {
	initMetrics()
	packetsCounter.Inc()
}

// IncErrors increments the aggregated error counter.
func IncErrors() {
	initMetrics()
	errorsCounter.Inc()
}

// IncDBWrites increments the successful database upsert counter.
func IncDBWrites() {
	initMetrics()
	dbWritesCounter.Inc()
}

// IncDBWriteErrors increments the failed database write counter.
func IncDBWriteErrors() {
	initMetrics()
	dbWriteErrorsCounter.Inc()
}

// ObserveDBWrite records a database write latency sample.
func ObserveDBWrite(d time.Duration) {
	initMetrics()
	dbWriteHist.Observe(d.Seconds())
}

// ObserveDBBatchSize records the number of measurements flushed in a batch.
func ObserveDBBatchSize(size int) {
	initMetrics()
	if size <= 0 {
		return
	}
	dbBatchSizeHist.Observe(float64(size))
}

// ObserveDBBatchWait records the time spent accumulating a batch before flushing.
func ObserveDBBatchWait(d time.Duration) {
	initMetrics()
	if d < 0 {
		d = 0
	}
	dbBatchWaitHist.Observe(d.Seconds())
}

// HTTPMiddleware instruments HTTP handlers with request/latency metrics.
func HTTPMiddleware(pathResolver func(*http.Request) string) func(http.Handler) http.Handler {
	initMetrics()

	if pathResolver == nil {
		pathResolver = func(r *http.Request) string {
			return r.URL.Path
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()

			next.ServeHTTP(recorder, r)

			duration := time.Since(start)
			path := pathResolver(r)
			if path == "" {
				path = "unknown"
			}

			method := r.Method
			status := strconv.Itoa(recorder.Status())

			httpRequests.WithLabelValues(path, method, status).Inc()
			httpDuration.WithLabelValues(path, method).Observe(duration.Seconds())

			if recorder.Status() >= http.StatusBadRequest {
				errorsCounter.Inc()
			}
		})
	}
}

// GRPCUnaryInterceptor instruments gRPC unary handlers with request/latency metrics.
func GRPCUnaryInterceptor() grpc.UnaryServerInterceptor {
	initMetrics()

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		method := info.FullMethod
		if method == "" {
			method = "unknown"
		}

		code := status.Code(err).String()
		grpcRequests.WithLabelValues(method, code).Inc()
		grpcDuration.WithLabelValues(method).Observe(duration.Seconds())

		if err != nil && status.Code(err) != codes.OK {
			errorsCounter.Inc()
		}

		return resp, err
	}
}

// statusRecorder captures the response status code for instrumentation.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Status() int {
	if s.status == 0 {
		return http.StatusOK
	}
	return s.status
}
