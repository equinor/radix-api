package deployments

import (
	"time"

	deploymentModels "github.com/statoil/radix-api/api/deployments/models"
	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
)

// Builder Builds DTOs
type Builder interface {
	withRadixDeployment(v1.RadixDeployment) Builder
	withName(string) Builder
	withAppName(string) Builder
	withEnvironment(string) Builder
	withActiveFrom(time.Time) Builder
	withActiveTo(time.Time) Builder
	withJobName(string) Builder
	withComponents([]v1.RadixDeployComponent) Builder
	buildApplicationDeployment() *deploymentModels.ApplicationDeployment
}

type builder struct {
	name        string
	appName     string
	environment string
	activeFrom  time.Time
	activeTo    time.Time
	jobName     string
	components  []*deploymentModels.ComponentSummary
}

func (b *builder) withRadixDeployment(rd v1.RadixDeployment) Builder {
	jobName := rd.Labels["radix-job-name"]

	b.withName(rd.GetName()).
		withAppName(rd.Spec.AppName).
		withEnvironment(rd.Spec.Environment).
		withActiveFrom(rd.CreationTimestamp.Time).
		withJobName(jobName).
		withComponents(rd.Spec.Components)

	return b
}

func (b *builder) withJobName(jobName string) Builder {
	b.jobName = jobName
	return b
}

func (b *builder) withComponents(components []v1.RadixDeployComponent) Builder {
	for _, component := range components {
		b.components = append(b.components, &deploymentModels.ComponentSummary{
			Name:  component.Name,
			Image: component.Image,
		})
	}

	return b
}

func (b *builder) withName(name string) Builder {
	b.name = name
	return b
}

func (b *builder) withAppName(appName string) Builder {
	b.appName = appName
	return b
}

func (b *builder) withEnvironment(environment string) Builder {
	b.environment = environment
	return b
}

func (b *builder) withActiveFrom(activeFrom time.Time) Builder {
	b.activeFrom = activeFrom
	return b
}

func (b *builder) withActiveTo(activeTo time.Time) Builder {
	b.activeTo = activeTo
	return b
}

func (b *builder) buildApplicationDeployment() *deploymentModels.ApplicationDeployment {
	return &deploymentModels.ApplicationDeployment{
		Name:        b.name,
		AppName:     b.appName,
		Environment: b.environment,
		ActiveFrom:  utils.FormatTimestamp(b.activeFrom),
		ActiveTo:    utils.FormatTimestamp(b.activeTo),
		Components:  b.components,
		JobName:     b.jobName,
	}
}

// NewBuilder Constructor for application builder
func NewBuilder() Builder {
	return &builder{}
}
