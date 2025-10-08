package promhttp

import (
	"net/http"

	"aggregator-service/app/src/infra/prometheus"
)

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		prometheus.Gather(w)
	})
}
