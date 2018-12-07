package models

import (
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
)

// ComponentBuilder Builds DTOs
type ComponentBuilder interface {
	WithPodNames([]string) ComponentBuilder
	WithReplicaSummaryList([]ReplicaSummary) ComponentBuilder
	WithRadixEnvironmentVariables(map[string]string) ComponentBuilder
	WithComponent(v1.RadixDeployComponent) ComponentBuilder
	BuildComponentSummary() *ComponentSummary
	BuildComponent() *Component
}

type componentBuilder struct {
	component                 v1.RadixDeployComponent
	podNames                  []string
	replicaSummaryList        []ReplicaSummary
	radixEnvironmentVariables map[string]string
	ports                     []Port
}

func (b *componentBuilder) WithPodNames(podNames []string) ComponentBuilder {
	b.podNames = podNames
	return b
}

func (b *componentBuilder) WithReplicaSummaryList(replicaSummaryList []ReplicaSummary) ComponentBuilder {
	b.replicaSummaryList = replicaSummaryList
	return b
}

func (b *componentBuilder) WithRadixEnvironmentVariables(radixEnvironmentVariables map[string]string) ComponentBuilder {
	b.radixEnvironmentVariables = radixEnvironmentVariables
	return b
}

func (b *componentBuilder) WithComponent(component v1.RadixDeployComponent) ComponentBuilder {
	b.component = component

	ports := []Port{}
	if component.Ports != nil {
		for _, port := range component.Ports {
			ports = append(ports, Port{
				Name: port.Name,
				Port: port.Port,
			})
		}
	}

	b.ports = ports
	return b
}

func (b *componentBuilder) BuildComponentSummary() *ComponentSummary {
	return &ComponentSummary{
		Name:  b.component.Name,
		Image: b.component.Image,
	}
}

func (b *componentBuilder) BuildComponent() *Component {
	secrets := b.component.Secrets
	if secrets == nil {
		secrets = []string{}
	}

	variables := v1.EnvVarsMap{}
	for name, value := range b.component.EnvironmentVariables {
		variables[name] = value
	}

	for name, value := range b.radixEnvironmentVariables {
		variables[name] = value
	}

	return &Component{
		Name:        b.component.Name,
		Image:       b.component.Image,
		Ports:       b.ports,
		Secrets:     secrets,
		Variables:   variables,
		Replicas:    b.podNames,
		ReplicaList: b.replicaSummaryList,
	}
}

// NewComponentBuilder Constructor for application component
func NewComponentBuilder() ComponentBuilder {
	return &componentBuilder{}
}
