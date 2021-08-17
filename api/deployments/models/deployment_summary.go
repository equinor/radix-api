package models

type DeploymentSummaryPipelineJobInfo struct {
	// Name of job creating deployment
	//
	// required: false
	CreatedByJob string `json:"createdByJob,omitempty"`

	// Type of pipeline job
	//
	// required: false
	// example: build-deploy
	PipelineJobType string `json:"pipelineJobType,omitempty"`

	// Name of the environment the deployment was promoted from
	// Applies only for pipeline jobs of type 'promote'
	//
	// required: false
	// example: qa
	PromotedFromEnvironment string `json:"promotedFromEnvironment,omitempty"`

	// CommitID the commit ID of the branch to build
	//
	// required: false
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	CommitID string `json:"commitID,omitempty"`
}

// DeploymentSummary describe an deployment
// swagger:model DeploymentSummary
type DeploymentSummary struct {
	DeploymentSummaryPipelineJobInfo `json:",inline"`

	// Name the unique name of the Radix application deployment
	//
	// required: false
	// example: radix-canary-golang-tzbqi
	Name string `json:"name"`

	// Environment the environment this Radix application deployment runs in
	//
	// required: false
	// example: prod
	Environment string `json:"environment"`

	// ActiveFrom Timestamp when the deployment starts (or created)
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	ActiveFrom string `json:"activeFrom"`

	// ActiveTo Timestamp when the deployment ends
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	ActiveTo string `json:"activeTo,omitempty"`
}
