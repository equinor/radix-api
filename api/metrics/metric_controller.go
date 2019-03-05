package metrics

import (
	"fmt"
	"net/http"
	"strings"

	apiutils "github.com/equinor/radix-api/api/utils"
)

const (
	appNameLabel        = "app_name"
	jobsTriggeredMetric = "jobs_triggered"
)

// GetMetrics Get logs of a job for an application
func GetMetrics(w http.ResponseWriter, r *http.Request) {
	monitor := apiutils.GetMonitor()
	jobsTriggered := monitor.GetJobsTriggered()

	for appName, numJobs := range jobsTriggered {
		labels := map[string]interface{}{
			appNameLabel: appName,
		}

		var labelsStr string

		for labelName, labelValue := range labels {
			labelsStr += fmt.Sprintf(`%s="%v",`, labelName, labelValue)
		}
		labelsStr = strings.Trim(labelsStr, ",")

		appMetrics := map[string]interface{}{
			jobsTriggeredMetric: numJobs,
		}

		for metric, value := range appMetrics {
			fmt.Fprintf(w, "%s{%s} %v\n", metric, labelsStr, value)
		}
	}
}
