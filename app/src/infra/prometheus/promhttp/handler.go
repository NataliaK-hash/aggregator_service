package promhttp

import (
	"net/http"

	"aggregator-service/app/src/infra/prometheus"
)

// Handler exposes a HTTP handler for registered metrics.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		prometheus.WriteTo(w)
	})
}
