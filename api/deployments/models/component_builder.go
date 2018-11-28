package models

import (
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
)

// ComponentBuilder Builds DTOs
type ComponentBuilder interface {
	WithComponent(v1.RadixDeployComponent) ComponentBuilder
	BuildComponentSummary() *ComponentSummary
	BuildComponentDeployment() *ComponentDeployment
}

type componentBuilder struct {
	component v1.RadixDeployComponent
	podNames  []string
}

func (b *componentBuilder) WithPodNames(podNames []string) ComponentBuilder {
	b.podNames = podNames
	return b
}

func (b *componentBuilder) WithComponent(component v1.RadixDeployComponent) ComponentBuilder {
	b.component = component
	return b
}

func (b *componentBuilder) BuildComponentSummary() *ComponentSummary {
	return &ComponentSummary{
		Name:  b.component.Name,
		Image: b.component.Image,
	}
}

func (b *componentBuilder) BuildComponentDeployment() *ComponentDeployment {
	secrets := b.component.Secrets
	if secrets == nil {
		secrets = []string{}
	}
	variables := b.component.EnvironmentVariables
	if variables == nil {
		variables = v1.EnvVarsMap{}
	}

	return &ComponentDeployment{
		Name:      b.component.Name,
		Image:     b.component.Image,
		Ports:     b.component.Ports,
		Secrets:   secrets,
		Variables: variables,
		Replicas:  b.podNames,
	}
}

// NewComponentBuilder Constructor for application component
func NewComponentBuilder() ComponentBuilder {
	return &componentBuilder{}
}
