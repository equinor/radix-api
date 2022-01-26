package models

import (
	"time"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// DeploymentBuilder Builds DTOs
type DeploymentBuilder interface {
	WithRadixDeployment(v1.RadixDeployment) (DeploymentBuilder, error)
	WithName(string) DeploymentBuilder
	WithEnvironment(string) DeploymentBuilder
	WithActiveFrom(time.Time) DeploymentBuilder
	WithActiveTo(time.Time) DeploymentBuilder
	WithJobName(string) DeploymentBuilder
	WithPipelineJob(*v1.RadixJob) DeploymentBuilder
	WithComponents(components []*Component) DeploymentBuilder
	BuildDeploymentSummary() *DeploymentSummary
	BuildDeployment() *Deployment
}

type deploymentBuilder struct {
	name        string
	environment string
	activeFrom  time.Time
	activeTo    time.Time
	jobName     string
	pipelineJob *v1.RadixJob
	components  []*Component
}

func (b *deploymentBuilder) WithRadixDeployment(rd v1.RadixDeployment) (DeploymentBuilder, error) {
	jobName := rd.Labels[kube.RadixJobNameLabel]

	components := make([]*Component, len(rd.Spec.Components))
	for i, component := range rd.Spec.Components {
		builder, err := NewComponentBuilder().WithComponent(&component)
		if err != nil {
			return nil, err
		}
		components[i] = builder.BuildComponent()
	}

	b.WithName(rd.GetName()).
		WithEnvironment(rd.Spec.Environment).
		WithJobName(jobName).
		WithComponents(components).
		WithActiveFrom(rd.Status.ActiveFrom.Time).
		WithActiveTo(rd.Status.ActiveTo.Time)

	return b, nil
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

func (b *deploymentBuilder) BuildDeploymentSummary() *DeploymentSummary {
	return &DeploymentSummary{
		Name:                             b.name,
		Environment:                      b.environment,
		ActiveFrom:                       radixutils.FormatTimestamp(b.activeFrom),
		ActiveTo:                         radixutils.FormatTimestamp(b.activeTo),
		DeploymentSummaryPipelineJobInfo: b.buildDeploySummaryPipelineJobInfo(),
	}
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

func (b *deploymentBuilder) BuildDeployment() *Deployment {
	return &Deployment{
		Name:         b.name,
		Environment:  b.environment,
		ActiveFrom:   radixutils.FormatTimestamp(b.activeFrom),
		ActiveTo:     radixutils.FormatTimestamp(b.activeTo),
		Components:   b.components,
		CreatedByJob: b.jobName,
	}
}

// NewDeploymentBuilder Constructor for application deploymentBuilder
func NewDeploymentBuilder() DeploymentBuilder {
	return &deploymentBuilder{}
}
