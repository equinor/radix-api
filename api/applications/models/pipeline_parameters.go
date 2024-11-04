package models

import (
	jobModels "github.com/equinor/radix-api/api/jobs/models"
)

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

	// ImageRepository of the component, without image name and image-tag
	//
	// example: ghcr.io/test
	ImageRepository string `json:"imageRepository,omitempty"`

	// ImageName of the component, without repository name and image-tag
	//
	// example: radix-component
	ImageName string `json:"imageName,omitempty"`

	// ImageTag of the image - if empty will use default logic
	//
	// example: master-latest
	ImageTag string `json:"imageTag,omitempty"`

	// OverrideUseBuildCache override default or configured build cache option
	//
	// required: false
	// Extensions:
	// x-nullable: true
	OverrideUseBuildCache *bool `json:"overrideUseBuildCache,omitempty"`

	// DeployExternalDNS deploy external DNS
	//
	// required: false
	// Extensions:
	// x-nullable: true
	DeployExternalDNS *bool `json:"deployExternalDNS,omitempty"`
}

// MapPipelineParametersBuildToJobParameter maps to JobParameter
func (buildParam PipelineParametersBuild) MapPipelineParametersBuildToJobParameter() *jobModels.JobParameters {
	return &jobModels.JobParameters{
		Branch:                buildParam.Branch,
		CommitID:              buildParam.CommitID,
		PushImage:             buildParam.PushImageToContainerRegistry(),
		TriggeredBy:           buildParam.TriggeredBy,
		ImageRepository:       buildParam.ImageRepository,
		ImageName:             buildParam.ImageName,
		ImageTag:              buildParam.ImageTag,
		OverrideUseBuildCache: buildParam.OverrideUseBuildCache,
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

	// Image tags names for components
	//
	// example: component1=tag1,component2=tag2
	ImageTagNames map[string]string `json:"imageTagNames"`

	// TriggeredBy of the job - if empty will use user token upn (user principle name)
	//
	// example: a_user@equinor.com
	TriggeredBy string `json:"triggeredBy,omitempty"`

	// CommitID the commit ID of the branch
	// OPTIONAL for information only
	//
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	CommitID string `json:"commitID,omitempty"`

	// ComponentsToDeploy List of components to deploy
	// OPTIONAL If specified, only these components are deployed
	//
	// required: false
	ComponentsToDeploy []string `json:"componentsToDeploy"`
}

// MapPipelineParametersDeployToJobParameter maps to JobParameter
func (deployParam PipelineParametersDeploy) MapPipelineParametersDeployToJobParameter() *jobModels.JobParameters {
	return &jobModels.JobParameters{
		ToEnvironment:      deployParam.ToEnvironment,
		TriggeredBy:        deployParam.TriggeredBy,
		ImageTagNames:      deployParam.ImageTagNames,
		CommitID:           deployParam.CommitID,
		ComponentsToDeploy: deployParam.ComponentsToDeploy,
	}
}

// PipelineParametersApplyConfig describes base info
// swagger:model PipelineParametersApplyConfig
type PipelineParametersApplyConfig struct {
	// TriggeredBy of the job - if empty will use user token upn (user principle name)
	//
	// example: a_user@equinor.com
	TriggeredBy string `json:"triggeredBy,omitempty"`

	// DeployExternalDNS deploy external DNS
	//
	// required: false
	// Extensions:
	// x-nullable: true
	DeployExternalDNS *bool `json:"deployExternalDNS,omitempty"`
}

// MapPipelineParametersApplyConfigToJobParameter maps to JobParameter
func (param PipelineParametersApplyConfig) MapPipelineParametersApplyConfigToJobParameter() *jobModels.JobParameters {
	return &jobModels.JobParameters{
		TriggeredBy:       param.TriggeredBy,
		DeployExternalDNS: param.DeployExternalDNS,
	}
}
