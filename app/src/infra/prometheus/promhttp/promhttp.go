package promhttp

import (
	"bytes"
	"net/http"

	"aggregator-service/app/src/infra/prometheus"
)

const fallbackMetrics = "# HELP metrics_placeholder Generated placeholder metrics when registry is empty\n# TYPE metrics_placeholder gauge\nmetrics_placeholder 0\n"

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		var buf bytes.Buffer
		prometheus.Gather(&buf)

		body := buf.String()
		if body == "" {
			body = fallbackMetrics
		}

		_, _ = w.Write([]byte(body))
	})
}
