package models

// JobParameters parameters to create a pipeline job
// Not exposed in the API
type JobParameters struct {
	// For build pipeline: Name of the branch
	Branch string `json:"branch"`

	// For build pipeline: Commit ID of the branch
	CommitID string `json:"commitID"`

	// For build pipeline: Should image be pushed to container registry
	PushImage bool `json:"pushImage"`

	// For promote pipeline: Name (ID) of deployment to promote
	DeploymentName string `json:"deploymentName"`

	// For promote pipeline: Environment to locate deployment to promote
	FromEnvironment string `json:"fromEnvironment"`

	// For promote pipeline: Target environment for promotion
	ToEnvironment string `json:"toEnvironment"`
}

// GetPushImageTag Represents boolean as 1 or 0
func (param JobParameters) GetPushImageTag() string {
	if param.PushImage {
		return "1"
	}

	return "0"
}
