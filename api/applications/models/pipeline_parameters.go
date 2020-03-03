package models

import jobModels "github.com/equinor/radix-api/api/jobs/models"

// PipelineParametersPromote identify deployment to promote and a target environment
// swagger:model PipelineParametersPromote
type PipelineParametersPromote struct {
	// ID of the deployment to promote
	// REQUIRED for "promote" pipeline
	//
	// example: dev-9tyu1-tftmnqzq
	DeploymentName string `json:"deploymentName"`

	// Name of environment where to look for the deployment to be promoted
	// REQUIRED for "promote" pipeline
	//
	// example: prod
	FromEnvironment string `json:"fromEnvironment"`

	// Name of environment to receive the promoted deployment
	// REQUIRED for "promote" pipeline
	//
	// example: prod
	ToEnvironment string `json:"toEnvironment"`

	// TriggeredBy of the job - if empty will use user token upn (user principle name)
	//
	// example: a_user@equinor.com
	TriggeredBy string `json:"triggeredBy,omitempty"`
}

// MapPipelineParametersPromoteToJobParameter maps to JobParameter
func (promoteParam PipelineParametersPromote) MapPipelineParametersPromoteToJobParameter() *jobModels.JobParameters {
	return &jobModels.JobParameters{
		DeploymentName:  promoteParam.DeploymentName,
		FromEnvironment: promoteParam.FromEnvironment,
		ToEnvironment:   promoteParam.ToEnvironment,
		TriggeredBy:     promoteParam.TriggeredBy,
	}
}

// PipelineParametersBuild describe branch to build and its commit ID
// swagger:model PipelineParametersBuild
type PipelineParametersBuild struct {
	// Branch the branch to build
	// REQUIRED for "build" and "build-deploy" pipelines
	//
	// example: master
	Branch string `json:"branch"`

	// CommitID the commit ID of the branch to build
	// REQUIRED for "build" and "build-deploy" pipelines
	//
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	CommitID string `json:"commitID"`

	// PushImage should image be pushed to container registry. Defaults pushing
	//
	// example: true
	PushImage string `json:"pushImage"`

	// TriggeredBy of the job - if empty will use user token upn (user principle name)
	//
	// example: a_user@equinor.com
	TriggeredBy string `json:"triggeredBy,omitempty"`
}

// MapPipelineParametersBuildToJobParameter maps to JobParameter
func (buildParam PipelineParametersBuild) MapPipelineParametersBuildToJobParameter() *jobModels.JobParameters {
	return &jobModels.JobParameters{
		Branch:      buildParam.Branch,
		CommitID:    buildParam.CommitID,
		PushImage:   buildParam.PushImageToContainerRegistry(),
		TriggeredBy: buildParam.TriggeredBy,
	}
}

// PushImageToContainerRegistry Normalises the "PushImage" param from a string
func (buildParam PipelineParametersBuild) PushImageToContainerRegistry() bool {
	return !(buildParam.PushImage == "0" || buildParam.PushImage == "false")
}

// PipelineParametersDeploy describes environment to deploy
// swagger:model PipelineParametersDeploy
type PipelineParametersDeploy struct {
	// Name of environment to deploy
	// REQUIRED for "deploy" pipeline
	//
	// example: prod
	ToEnvironment string `json:"toEnvironment"`

	// TriggeredBy of the job - if empty will use user token upn (user principle name)
	//
	// example: a_user@equinor.com
	TriggeredBy string `json:"triggeredBy,omitempty"`
}

// MapPipelineParametersDeployToJobParameter maps to JobParameter
func (deployParam PipelineParametersDeploy) MapPipelineParametersDeployToJobParameter() *jobModels.JobParameters {
	return &jobModels.JobParameters{
		ToEnvironment: deployParam.ToEnvironment,
		TriggeredBy:   deployParam.TriggeredBy,
	}
}
