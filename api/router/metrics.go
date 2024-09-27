package router

import (
	"net/http"

	"github.com/equinor/radix-api/api/middleware/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/negroni/v3"
)

// NewMetricsHandler Constructor function
func NewMetricsHandler() http.Handler {
	serveMux := http.NewServeMux()
	serveMux.Handle("GET /metrics", promhttp.Handler())

	n := negroni.New(
		recovery.CreateMiddleware(),
		logger.CreateZerologRequestIdMiddleware(),
		logger.CreateZerologRequestDetailsMiddleware(),
		logger.CreateZerologRequestLoggerMiddleware(),
	)
	n.UseHandler(serveMux)

	return n
}
