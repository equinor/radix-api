package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	jobsTriggeredMetric = "radix_api_jobs_triggered"
	appNameLabel        = "app_name"
	pipelineLabel       = "pipeline"
)

var (
	nrJobsTriggered = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: jobsTriggeredMetric,
		Help: "The total number of jobs triggered",
	}, []string{appNameLabel, pipelineLabel})
)

// AddJobTriggered New job triggered for application
func AddJobTriggered(appName, pipeline string) {
	nrJobsTriggered.With(prometheus.Labels{appNameLabel: appName, pipelineLabel: pipeline}).Inc()
}
