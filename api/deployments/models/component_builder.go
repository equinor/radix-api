package models

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const (
	certPartSuffix = "-cert"
	keyPartSuffix  = "-key"
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
	componentName             string
	componentImage            string
	podNames                  []string
	replicaSummaryList        []ReplicaSummary
	environmentVariables      map[string]string
	radixEnvironmentVariables map[string]string
	secrets                   []string
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
	b.componentName = component.Name
	b.componentImage = component.Image

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
	b.secrets = component.Secrets
	if b.secrets == nil {
		b.secrets = []string{}
	}

	for _, externalAlias := range component.DNSExternalAlias {
		b.secrets = append(b.secrets, externalAlias+certPartSuffix)
		b.secrets = append(b.secrets, externalAlias+keyPartSuffix)
	}

	b.environmentVariables = component.EnvironmentVariables
	return b
}

func (b *componentBuilder) BuildComponentSummary() *ComponentSummary {
	return &ComponentSummary{
		Name:  b.componentName,
		Image: b.componentImage,
	}
}

func (b *componentBuilder) BuildComponent() *Component {
	variables := v1.EnvVarsMap{}
	for name, value := range b.environmentVariables {
		variables[name] = value
	}

	for name, value := range b.radixEnvironmentVariables {
		variables[name] = value
	}

	return &Component{
		Name:        b.componentName,
		Image:       b.componentImage,
		Ports:       b.ports,
		Secrets:     b.secrets,
		Variables:   variables,
		Replicas:    b.podNames,
		ReplicaList: b.replicaSummaryList,
	}
}

// NewComponentBuilder Constructor for application component
func NewComponentBuilder() ComponentBuilder {
	return &componentBuilder{}
}
