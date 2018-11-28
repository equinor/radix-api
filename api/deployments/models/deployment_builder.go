package models

import (
	"time"

	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
)

// DeploymentBuilder Builds DTOs
type DeploymentBuilder interface {
	WithRadixDeployment(v1.RadixDeployment) DeploymentBuilder
	WithName(string) DeploymentBuilder
	WithAppName(string) DeploymentBuilder
	WithEnvironment(string) DeploymentBuilder
	WithActiveFrom(time.Time) DeploymentBuilder
	WithActiveTo(time.Time) DeploymentBuilder
	WithJobName(string) DeploymentBuilder
	WithComponents(components []ComponentBuilder) DeploymentBuilder
	BuildDeploymentSummary() *DeploymentSummary
	BuildDeployment() *Deployment
}

type deploymentBuilder struct {
	name        string
	appName     string
	environment string
	activeFrom  time.Time
	activeTo    time.Time
	jobName     string
	components  []ComponentBuilder
}

func (b *deploymentBuilder) WithRadixDeployment(rd v1.RadixDeployment) DeploymentBuilder {
	jobName := rd.Labels["radix-job-name"]

	components := make([]ComponentBuilder, 0)
	for _, component := range rd.Spec.Components {
		components = append(components, NewComponentBuilder().WithComponent(component))
	}

	b.
		WithName(rd.GetName()).
		WithAppName(rd.Spec.AppName).
		WithEnvironment(rd.Spec.Environment).
		WithActiveFrom(rd.CreationTimestamp.Time).
		WithJobName(jobName).
		WithComponents(components)

	return b
}

func (b *deploymentBuilder) WithJobName(jobName string) DeploymentBuilder {
	b.jobName = jobName
	return b
}

func (b *deploymentBuilder) WithComponents(components []ComponentBuilder) DeploymentBuilder {
	b.components = components
	return b
}

func (b *deploymentBuilder) WithName(name string) DeploymentBuilder {
	b.name = name
	return b
}

func (b *deploymentBuilder) WithAppName(appName string) DeploymentBuilder {
	b.appName = appName
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
		Name:         b.name,
		Environment:  b.environment,
		ActiveFrom:   utils.FormatTimestamp(b.activeFrom),
		ActiveTo:     utils.FormatTimestamp(b.activeTo),
		CreatedByJob: b.jobName,
	}
}

func (b *deploymentBuilder) BuildDeployment() *Deployment {
	components := make([]*ComponentDeployment, len(b.components))
	for _, component := range b.components {
		components = append(components, component.BuildComponentDeployment())
	}

	return &Deployment{
		Name:         b.name,
		Environment:  b.environment,
		ActiveFrom:   utils.FormatTimestamp(b.activeFrom),
		ActiveTo:     utils.FormatTimestamp(b.activeTo),
		Components:   components,
		CreatedByJob: b.jobName,
	}
}

// NewDeploymentBuilder Constructor for application deploymentBuilder
func NewDeploymentBuilder() DeploymentBuilder {
	return &deploymentBuilder{}
}
