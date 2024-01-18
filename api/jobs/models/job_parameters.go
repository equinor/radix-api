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

	// TriggeredBy of the job - if empty will use user token upn (user principle name)
	TriggeredBy string `json:"triggeredBy"`

	// For promote pipeline: Name (ID) of deployment to promote
	DeploymentName string `json:"deploymentName"`

	// For promote pipeline: Environment to locate deployment to promote
	FromEnvironment string `json:"fromEnvironment"`

	// For promote pipeline: Target environment for promotion
	ToEnvironment string `json:"toEnvironment"`

	// ImageRepository of the component, without image name and image-tag
	ImageRepository string

	// ImageName of the component, without repository name and image-tag
	ImageName string

	// ImageTag of the image - if empty will use default logic
	ImageTag string

	// ImageTagNames tags for components - if empty will use default logic
	//
	// example: component1=tag1,component2=tag2
	ImageTagNames map[string]string

	// Deployment should not be applied
	PostponeDeployment bool `json:"postponedDeployment"`
}

// GetPushImageTag Represents boolean as 1 or 0
func (param JobParameters) GetPushImageTag() string {
	if param.PushImage {
		return "1"
	}

	return "0"
}
