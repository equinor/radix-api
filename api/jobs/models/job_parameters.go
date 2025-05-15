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

	// For build or promote pipeline: Target environment for building and promotion
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

	// ComponentsToDeploy List of components to deploy
	// OPTIONAL If specified, only these components are deployed
	//
	// required: false
	ComponentsToDeploy []string `json:"componentsToDeploy"`

	// OverrideUseBuildCache override default or configured build cache option
	//
	// required: false
	// Extensions:
	// x-nullable: true
	OverrideUseBuildCache *bool `json:"overrideUseBuildCache,omitempty"`

	// RefreshBuildCache forces to rebuild cache when UseBuildCache is true in the RadixApplication or OverrideUseBuildCache is true
	//
	// required: false
	// Extensions:
	// x-nullable: true
	RefreshBuildCache *bool `json:"refreshBuildCache,omitempty"`

	// DeployExternalDNS deploy external DNS
	//
	// required: false
	// Extensions:
	// x-nullable: true
	DeployExternalDNS *bool `json:"deployExternalDNS,omitempty"`
}

// GetPushImageTag Represents boolean as 1 or 0
func (param JobParameters) GetPushImageTag() string {
	if param.PushImage {
		return "1"
	}

	return "0"
}
