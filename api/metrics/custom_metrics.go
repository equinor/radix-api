package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	jobsTriggeredMetric         = "radix_api_jobs_triggered"
	requestDurationMetric       = "radix_api_request_duration_seconds_sum"
	requestDurationBucketMetric = "radix_api_request_duration_seconds"

	appNameLabel  = "app_name"
	pipelineLabel = "pipeline"
	pathLabel     = "path"
	methodLabel   = "method"
)

var (
	nrJobsTriggered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: jobsTriggeredMetric,
			Help: "The total number of jobs triggered",
		}, []string{appNameLabel, pipelineLabel})
	resTime = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: requestDurationMetric,
			Help: "Request duration seconds",
		},
		[]string{pathLabel, methodLabel},
	)
	resTimeBucket = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    requestDurationBucketMetric,
			Help:    "Request duration seconds bucket",
			Buckets: []float64{1, 2, 5, 6, 10},
		},
		[]string{pathLabel, methodLabel},
	)
)

// AddJobTriggered New job triggered for application
func AddJobTriggered(appName, pipeline string) {
	nrJobsTriggered.With(prometheus.Labels{appNameLabel: appName, pipelineLabel: pipeline}).Inc()
}

// AddRequestDuration Add request duration for given endpoint
func AddRequestDuration(path, method string, duration time.Duration) {
	resTime.With(prometheus.Labels{pathLabel: path, methodLabel: method}).Observe(duration.Seconds())
	resTimeBucket.With(prometheus.Labels{pathLabel: path, methodLabel: method}).Observe(duration.Seconds())
}
