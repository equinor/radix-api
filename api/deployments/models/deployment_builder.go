package models

import (
	"time"

	radixutils "github.com/equinor/radix-common/utils"
	errorutils "github.com/equinor/radix-common/utils/errors"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// DeploymentBuilder Builds DTOs
type DeploymentBuilder interface {
	WithRadixDeployment(v1.RadixDeployment) DeploymentBuilder
	WithName(string) DeploymentBuilder
	WithEnvironment(string) DeploymentBuilder
	WithActiveFrom(time.Time) DeploymentBuilder
	WithActiveTo(time.Time) DeploymentBuilder
	WithJobName(string) DeploymentBuilder
	WithPipelineJob(*v1.RadixJob) DeploymentBuilder
	WithComponents(components []*Component) DeploymentBuilder
	BuildDeploymentSummary() (*DeploymentSummary, error)
	BuildDeployment() (*Deployment, error)
}

type deploymentBuilder struct {
	name        string
	environment string
	activeFrom  time.Time
	activeTo    time.Time
	jobName     string
	pipelineJob *v1.RadixJob
	components  []*Component
	errors      []error
}

func (b *deploymentBuilder) WithRadixDeployment(rd v1.RadixDeployment) DeploymentBuilder {
	jobName := rd.Labels[kube.RadixJobNameLabel]

	components := make([]*Component, len(rd.Spec.Components))
	for i, component := range rd.Spec.Components {
		componentDto, err := NewComponentBuilder().WithComponent(&component).BuildComponent()
		if err != nil {
			b.errors = append(b.errors, err)
			continue
		}
		components[i] = componentDto
	}

	b.WithName(rd.GetName()).
		WithEnvironment(rd.Spec.Environment).
		WithJobName(jobName).
		WithComponents(components).
		WithActiveFrom(rd.Status.ActiveFrom.Time).
		WithActiveTo(rd.Status.ActiveTo.Time)

	return b
}

func (b *deploymentBuilder) WithJobName(jobName string) DeploymentBuilder {
	b.jobName = jobName
	return b
}

func (b *deploymentBuilder) WithPipelineJob(job *v1.RadixJob) DeploymentBuilder {
	if job != nil {
		b.WithJobName(job.Name)
	}

	b.pipelineJob = job
	return b
}

func (b *deploymentBuilder) WithComponents(components []*Component) DeploymentBuilder {
	b.components = components
	return b
}

func (b *deploymentBuilder) WithName(name string) DeploymentBuilder {
	b.name = name
	return b
}

func (b *deploymentBuilder) WithEnvironment(environment string) DeploymentBuilder {
	b.environment = environment
	return b
}

func (b *deploymentBuilder) WithActiveFrom(activeFrom time.Time) DeploymentBuilder {
	b.activeFrom = activeFrom
	return b
}

func (b *deploymentBuilder) WithActiveTo(activeTo time.Time) DeploymentBuilder {
	b.activeTo = activeTo
	return b
}

func (b *deploymentBuilder) buildError() error {
	if len(b.errors) == 0 {
		return nil
	}

	return errorutils.Concat(b.errors)
}

func (b *deploymentBuilder) BuildDeploymentSummary() (*DeploymentSummary, error) {
	return &DeploymentSummary{
		Name:                             b.name,
		Environment:                      b.environment,
		ActiveFrom:                       radixutils.FormatTimestamp(b.activeFrom),
		ActiveTo:                         radixutils.FormatTimestamp(b.activeTo),
		DeploymentSummaryPipelineJobInfo: b.buildDeploySummaryPipelineJobInfo(),
	}, b.buildError()
}

func (b *deploymentBuilder) buildDeploySummaryPipelineJobInfo() DeploymentSummaryPipelineJobInfo {
	jobInfo := DeploymentSummaryPipelineJobInfo{
		CreatedByJob: b.jobName,
	}

	if b.pipelineJob != nil {
		jobInfo.CommitID = b.pipelineJob.Spec.Build.CommitID
		jobInfo.PipelineJobType = string(b.pipelineJob.Spec.PipeLineType)
		jobInfo.PromotedFromEnvironment = b.pipelineJob.Spec.Promote.FromEnvironment
	}

	return jobInfo
}

func (b *deploymentBuilder) BuildDeployment() (*Deployment, error) {
	return &Deployment{
		Name:         b.name,
		Environment:  b.environment,
		ActiveFrom:   radixutils.FormatTimestamp(b.activeFrom),
		ActiveTo:     radixutils.FormatTimestamp(b.activeTo),
		Components:   b.components,
		CreatedByJob: b.jobName,
	}, b.buildError()
}

// NewDeploymentBuilder Constructor for application deploymentBuilder
func NewDeploymentBuilder() DeploymentBuilder {
	return &deploymentBuilder{}
}
