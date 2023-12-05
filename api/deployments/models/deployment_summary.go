package models

type DeploymentSummaryPipelineJobInfo struct {
	// Name of job creating deployment
	//
	// required: false
	CreatedByJob string `json:"createdByJob,omitempty"`

	// Type of pipeline job
	//
	// required: false
	// enum: build,build-deploy,promote,deploy
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
	// required: true
	// example: radix-canary-golang-tzbqi
	Name string `json:"name"`

	// Array of component summaries
	//
	// required: false
	Components []*ComponentSummary `json:"components,omitempty"`

	// Environment the environment this Radix application deployment runs in
	//
	// required: true
	// example: prod
	Environment string `json:"environment"`

	// ActiveFrom Timestamp when the deployment starts (or created)
	//
	// required: true
	// example: 2006-01-02T15:04:05Z
	ActiveFrom string `json:"activeFrom"`

	// ActiveTo Timestamp when the deployment ends
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	ActiveTo string `json:"activeTo,omitempty"`

	// GitCommitHash the hash of the git commit from which radixconfig.yaml was parsed
	//
	// required: false
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	GitCommitHash string `json:"gitCommitHash,omitempty"`

	// GitTags the git tags that the git commit hash points to
	//
	// required: false
	// example: "v1.22.1 v1.22.3"
	GitTags string `json:"gitTags,omitempty"`
}

// DeploymentItem describe a deployment short info
// swagger:model DeploymentItem
type DeploymentItem struct {
	// Name the unique name of the Radix application deployment
	//
	// required: true
	// example: radix-canary-golang-tzbqi
	Name string `json:"name"`

	// ActiveFrom Timestamp when the deployment starts (or created)
	//
	// required: true
	// example: 2006-01-02T15:04:05Z
	ActiveFrom string `json:"activeFrom"`

	// ActiveTo Timestamp when the deployment ends
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	ActiveTo string `json:"activeTo,omitempty"`

	// GitCommitHash the hash of the git commit from which radixconfig.yaml was parsed
	//
	// required: false
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	GitCommitHash string `json:"gitCommitHash,omitempty"`
}
