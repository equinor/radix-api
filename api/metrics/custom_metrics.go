package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	appNameLabel        = "app_name"
	jobsTriggeredMetric = "jobs_triggered"
)

var (
	nrJobsTriggered = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "radix_api_jobs_triggered",
		Help: "The total number of jobs triggered",
	}, []string{"app_name"})
)

// AddJobTriggered New job triggered for application
func AddJobTriggered(appName string) {
	nrJobsTriggered.With(prometheus.Labels{"app_name": appName}).Inc()
}
