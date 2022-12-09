package models

import (
	"time"

	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"

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
	WithComponentSummaries(componentSummaries []*ComponentSummary) DeploymentBuilder
	BuildDeploymentSummary() (*DeploymentSummary, error)
	BuildDeployment() (*Deployment, error)
	WithGitCommitHash(string) DeploymentBuilder
	WithGitTags(string) DeploymentBuilder
	WithRadixRegistration(*v1.RadixRegistration) DeploymentBuilder
}

type deploymentBuilder struct {
	name               string
	environment        string
	activeFrom         time.Time
	activeTo           time.Time
	jobName            string
	pipelineJob        *v1.RadixJob
	components         []*Component
	componentSummaries []*ComponentSummary
	errors             []error
	gitCommitHash      string
	gitTags            string
	repository         string
}

func (b *deploymentBuilder) WithRadixDeployment(rd v1.RadixDeployment) DeploymentBuilder {
	jobName := rd.Labels[kube.RadixJobNameLabel]

	components := make([]*ComponentSummary, 0, len(rd.Spec.Components)+len(rd.Spec.Components))
	for _, component := range rd.Spec.Components {
		componentDto, err := NewComponentBuilder().WithComponent(&component).BuildComponentSummary()
		if err != nil {
			b.errors = append(b.errors, err)
			continue
		}
		components = append(components, componentDto)
	}
	for _, component := range rd.Spec.Jobs {
		componentDto, err := NewComponentBuilder().WithComponent(&component).BuildComponentSummary()
		if err != nil {
			b.errors = append(b.errors, err)
			continue
		}
		components = append(components, componentDto)
	}

	b.WithName(rd.GetName()).
		WithEnvironment(rd.Spec.Environment).
		WithComponentSummaries(components).
		WithJobName(jobName).
		WithActiveFrom(rd.Status.ActiveFrom.Time).
		WithActiveTo(rd.Status.ActiveTo.Time).
		WithGitCommitHash(rd.Annotations[kube.RadixCommitAnnotation]).
		WithGitTags(rd.Annotations[kube.RadixGitTagsAnnotation])

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

func (b *deploymentBuilder) WithComponentSummaries(componentSummaries []*ComponentSummary) DeploymentBuilder {
	b.componentSummaries = componentSummaries
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

func (b *deploymentBuilder) WithGitCommitHash(gitCommitHash string) DeploymentBuilder {
	b.gitCommitHash = gitCommitHash
	return b
}

func (b *deploymentBuilder) WithGitTags(gitTags string) DeploymentBuilder {
	b.gitTags = gitTags
	return b
}

func (b *deploymentBuilder) WithRadixRegistration(rr *v1.RadixRegistration) DeploymentBuilder {
	gitCloneUrl := rr.Spec.CloneURL
	b.repository = crdUtils.GetGithubRepositoryURLFromCloneURL(gitCloneUrl)
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
		Components:                       b.componentSummaries,
		Environment:                      b.environment,
		ActiveFrom:                       radixutils.FormatTimestamp(b.activeFrom),
		ActiveTo:                         radixutils.FormatTimestamp(b.activeTo),
		DeploymentSummaryPipelineJobInfo: b.buildDeploySummaryPipelineJobInfo(),
		GitCommitHash:                    b.gitCommitHash,
		GitTags:                          b.gitTags,
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
		Name:          b.name,
		Environment:   b.environment,
		ActiveFrom:    radixutils.FormatTimestamp(b.activeFrom),
		ActiveTo:      radixutils.FormatTimestamp(b.activeTo),
		Components:    b.components,
		CreatedByJob:  b.jobName,
		GitCommitHash: b.gitCommitHash,
		GitTags:       b.gitTags,
		Repository:    b.repository,
	}, b.buildError()
}

// NewDeploymentBuilder Constructor for application deploymentBuilder
func NewDeploymentBuilder() DeploymentBuilder {
	return &deploymentBuilder{}
}
