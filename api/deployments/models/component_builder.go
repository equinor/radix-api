package models

import (
	"github.com/equinor/radix-api/api/secrets/suffix"
	errorutils "github.com/equinor/radix-common/utils/errors"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
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
	BuildComponentSummary() (*ComponentSummary, error)
	BuildComponent() (*Component, error)
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
	errors                    []error
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
	b.componentType = string(component.GetType())
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
		b.secrets = append(b.secrets, externalAlias+suffix.ExternalDNSCert)
		b.secrets = append(b.secrets, externalAlias+suffix.ExternalDNSKeyPart)
	}

	for _, volumeMount := range component.GetVolumeMounts() {
		switch volumeMount.Type {
		case v1.MountTypeBlob:
			secretName := defaults.GetBlobFuseCredsSecretName(component.GetName(), volumeMount.Name)
			b.secrets = append(b.secrets, secretName+defaults.BlobFuseCredsAccountKeyPartSuffix)
			b.secrets = append(b.secrets, secretName+defaults.BlobFuseCredsAccountNamePartSuffix)
		case v1.MountTypeBlobCsiAzure, v1.MountTypeFileCsiAzure:
			secretName := defaults.GetCsiAzureCredsSecretName(component.GetName(), volumeMount.Name)
			b.secrets = append(b.secrets, secretName+defaults.CsiAzureCredsAccountKeyPartSuffix)
			b.secrets = append(b.secrets, secretName+defaults.CsiAzureCredsAccountNamePartSuffix)
		}
	}

	secretRef := component.GetSecretRefs()
	if secretRef.AzureKeyVaults != nil {
		for _, azureKeyVault := range secretRef.AzureKeyVaults {
			secretName := defaults.GetCsiAzureKeyVaultCredsSecretName(component.GetName(), azureKeyVault.Name)
			b.secrets = append(b.secrets, secretName+defaults.CsiAzureKeyVaultCredsClientIdSuffix)
			b.secrets = append(b.secrets, secretName+defaults.CsiAzureKeyVaultCredsClientSecretSuffix)
			for _, item := range azureKeyVault.Items {
				b.secrets = append(b.secrets, item.EnvVar)
			}
		}
	}

	if auth := component.GetAuthentication(); auth != nil && component.IsPublic() {
		if deployment.IsSecretRequiredForClientCertificate(auth.ClientCertificate) {
			b.secrets = append(b.secrets, utils.GetComponentClientCertificateSecretName(component.GetName()))
		}
		if auth.OAuth2 != nil {
			oauth2, err := defaults.NewOAuth2Config(defaults.WithOAuth2Defaults()).MergeWith(auth.OAuth2)
			if err != nil {
				b.errors = append(b.errors, err)
			}
			b.secrets = append(b.secrets, component.GetName()+suffix.OAuth2ClientSecret)
			b.secrets = append(b.secrets, component.GetName()+suffix.OAuth2CookieSecret)

			if oauth2.SessionStoreType == v1.SessionStoreRedis {
				b.secrets = append(b.secrets, component.GetName()+suffix.OAuth2RedisPassword)
			}
		}
	}

	b.environmentVariables = component.GetEnvironmentVariables()
	return b
}

func (b *componentBuilder) buildError() error {
	if len(b.errors) == 0 {
		return nil
	}

	return errorutils.Concat(b.errors)
}

func (b *componentBuilder) BuildComponentSummary() (*ComponentSummary, error) {
	return &ComponentSummary{
		Name:  b.componentName,
		Type:  b.componentType,
		Image: b.componentImage,
	}, b.buildError()
}

func (b *componentBuilder) BuildComponent() (*Component, error) {
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
	}, b.buildError()
}

// NewComponentBuilder Constructor for application component
func NewComponentBuilder() ComponentBuilder {
	return &componentBuilder{}
}
