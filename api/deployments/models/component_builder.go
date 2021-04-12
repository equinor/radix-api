package models

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

const (
	certPartSuffix = "-cert"
	keyPartSuffix  = "-key"
)

// ComponentBuilder Builds DTOs
type ComponentBuilder interface {
	WithStatus(ComponentStatus) ComponentBuilder
	WithPodNames([]string) ComponentBuilder
	WithReplicaSummaryList([]ReplicaSummary) ComponentBuilder
	WithScheduledJobSummaryList([]ScheduledJobSummary) ComponentBuilder
	WithSchedulerPort(schedulerPort *int32) ComponentBuilder
	WithScheduledJobPayloadPath(scheduledJobPayloadPath string) ComponentBuilder
	WithRadixEnvironmentVariables(map[string]string) ComponentBuilder
	WithComponent(v1.RadixCommonDeployComponent) ComponentBuilder
	BuildComponentSummary() *ComponentSummary
	BuildComponent() *Component
}

type componentBuilder struct {
	componentName             string
	componentType             string
	status                    ComponentStatus
	componentImage            string
	podNames                  []string
	replicaSummaryList        []ReplicaSummary
	scheduledJobSummaryList   []ScheduledJobSummary
	environmentVariables      map[string]string
	radixEnvironmentVariables map[string]string
	secrets                   []string
	ports                     []Port
	schedulerPort             *int32
	scheduledJobPayloadPath   string
}

func (b *componentBuilder) WithStatus(status ComponentStatus) ComponentBuilder {
	b.status = status
	return b
}

func (b *componentBuilder) WithPodNames(podNames []string) ComponentBuilder {
	b.podNames = podNames
	return b
}

func (b *componentBuilder) WithReplicaSummaryList(replicaSummaryList []ReplicaSummary) ComponentBuilder {
	b.replicaSummaryList = replicaSummaryList
	return b
}

func (b *componentBuilder) WithScheduledJobSummaryList(scheduledJobSummaryList []ScheduledJobSummary) ComponentBuilder {
	b.scheduledJobSummaryList = scheduledJobSummaryList
	return b
}

func (b *componentBuilder) WithRadixEnvironmentVariables(radixEnvironmentVariables map[string]string) ComponentBuilder {
	b.radixEnvironmentVariables = radixEnvironmentVariables
	return b
}

func (b *componentBuilder) WithSchedulerPort(schedulerPort *int32) ComponentBuilder {
	b.schedulerPort = schedulerPort
	return b
}

func (b *componentBuilder) WithScheduledJobPayloadPath(scheduledJobPayloadPath string) ComponentBuilder {
	b.scheduledJobPayloadPath = scheduledJobPayloadPath
	return b
}

func (b *componentBuilder) WithComponent(component v1.RadixCommonDeployComponent) ComponentBuilder {
	b.componentName = component.GetName()
	b.componentType = component.GetType()
	b.componentImage = component.GetImage()

	ports := []Port{}
	if component.GetPorts() != nil {
		for _, port := range component.GetPorts() {
			ports = append(ports, Port{
				Name: port.Name,
				Port: port.Port,
			})
		}
	}

	b.ports = ports
	b.secrets = component.GetSecrets()
	if b.secrets == nil {
		b.secrets = []string{}
	}

	for _, externalAlias := range component.GetDNSExternalAlias() {
		b.secrets = append(b.secrets, externalAlias+certPartSuffix)
		b.secrets = append(b.secrets, externalAlias+keyPartSuffix)
	}

	for _, volumeMount := range component.GetVolumeMounts() {
		if volumeMount.Type == v1.MountTypeBlob {
			secretName := defaults.GetBlobFuseCredsSecretName(component.GetName(), volumeMount.Name)
			b.secrets = append(b.secrets, secretName+defaults.BlobFuseCredsAccountKeyPartSuffix)
			b.secrets = append(b.secrets, secretName+defaults.BlobFuseCredsAccountNamePartSuffix)
		}
	}

	b.environmentVariables = *component.GetEnvironmentVariables()
	return b
}

func (b *componentBuilder) BuildComponentSummary() *ComponentSummary {
	return &ComponentSummary{
		Name:  b.componentName,
		Type:  b.componentType,
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
		Name:                    b.componentName,
		Type:                    b.componentType,
		Status:                  b.status.String(),
		Image:                   b.componentImage,
		Ports:                   b.ports,
		Secrets:                 b.secrets,
		Variables:               variables,
		Replicas:                b.podNames,
		ReplicaList:             b.replicaSummaryList,
		ScheduledJobList:        b.scheduledJobSummaryList,
		SchedulerPort:           b.schedulerPort,
		ScheduledJobPayloadPath: b.scheduledJobPayloadPath,
	}
}

// NewComponentBuilder Constructor for application component
func NewComponentBuilder() ComponentBuilder {
	return &componentBuilder{}
}
