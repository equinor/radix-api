package models

// ScheduledJobRequest holds information about a creating scheduled job request
// swagger:model ScheduledJobRequest
type ScheduledJobRequest struct {
	// Name of the Radix deployment for a job
	DeploymentName string `json:"deploymentName"`
}
