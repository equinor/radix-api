package secrets

import (
	"fmt"
	"github.com/equinor/radix-api/api/deployments"
	"github.com/equinor/radix-api/api/events"
	"github.com/equinor/radix-api/api/secrets/models"
	apiModels "github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	"testing"
)

func TestInit(t *testing.T) {
	type args struct {
		opts []SecretHandlerOptions
	}
	var tests []struct {
		name string
		args args
		want SecretHandler
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, Init(tt.args.opts...), "Init()")
		})
	}
}

func TestSecretHandler_ChangeComponentSecret(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		appName         string
		envName         string
		componentName   string
		secretName      string
		componentSecret models.SecretParameters
	}
	var tests []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			tt.wantErr(t, eh.ChangeComponentSecret(tt.args.appName, tt.args.envName, tt.args.componentName, tt.args.secretName, tt.args.componentSecret), fmt.Sprintf("ChangeComponentSecret(%v, %v, %v, %v, %v)", tt.args.appName, tt.args.envName, tt.args.componentName, tt.args.secretName, tt.args.componentSecret))
		})
	}
}

func TestSecretHandler_GetSecrets(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		appName string
		envName string
	}
	var tests []struct {
		name    string
		fields  fields
		args    args
		want    []models.Secret
		wantErr assert.ErrorAssertionFunc
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			got, err := eh.GetSecrets(tt.args.appName, tt.args.envName)
			if !tt.wantErr(t, err, fmt.Sprintf("GetSecrets(%v, %v)", tt.args.appName, tt.args.envName)) {
				return
			}
			assert.Equalf(t, tt.want, got, "GetSecrets(%v, %v)", tt.args.appName, tt.args.envName)
		})
	}
}

func TestSecretHandler_GetSecretsForDeployment(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		appName        string
		envName        string
		deploymentName string
	}
	var tests []struct {
		name    string
		fields  fields
		args    args
		want    []models.Secret
		wantErr assert.ErrorAssertionFunc
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			got, err := eh.GetSecretsForDeployment(tt.args.appName, tt.args.envName, tt.args.deploymentName)
			if !tt.wantErr(t, err, fmt.Sprintf("GetSecretsForDeployment(%v, %v, %v)", tt.args.appName, tt.args.envName, tt.args.deploymentName)) {
				return
			}
			assert.Equalf(t, tt.want, got, "GetSecretsForDeployment(%v, %v, %v)", tt.args.appName, tt.args.envName, tt.args.deploymentName)
		})
	}
}

func TestSecretHandler_getAzureVolumeMountSecrets(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		envNamespace          string
		component             v1.RadixCommonDeployComponent
		secretName            string
		volumeMountName       string
		accountNamePart       string
		accountKeyPart        string
		accountNamePartSuffix string
		accountKeyPartSuffix  string
		secretType            models.SecretType
	}
	var tests []struct {
		name   string
		fields fields
		args   args
		want   models.Secret
		want1  models.Secret
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			got, got1 := eh.getAzureVolumeMountSecrets(tt.args.envNamespace, tt.args.component, tt.args.secretName, tt.args.volumeMountName, tt.args.accountNamePart, tt.args.accountKeyPart, tt.args.accountNamePartSuffix, tt.args.accountKeyPartSuffix, tt.args.secretType)
			assert.Equalf(t, tt.want, got, "getAzureVolumeMountSecrets(%v, %v, %v, %v, %v, %v, %v, %v, %v)", tt.args.envNamespace, tt.args.component, tt.args.secretName, tt.args.volumeMountName, tt.args.accountNamePart, tt.args.accountKeyPart, tt.args.accountNamePartSuffix, tt.args.accountKeyPartSuffix, tt.args.secretType)
			assert.Equalf(t, tt.want1, got1, "getAzureVolumeMountSecrets(%v, %v, %v, %v, %v, %v, %v, %v, %v)", tt.args.envNamespace, tt.args.component, tt.args.secretName, tt.args.volumeMountName, tt.args.accountNamePart, tt.args.accountKeyPart, tt.args.accountNamePartSuffix, tt.args.accountKeyPartSuffix, tt.args.secretType)
		})
	}
}

func TestSecretHandler_getBlobFuseSecrets(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		component    v1.RadixCommonDeployComponent
		envNamespace string
		volumeMount  v1.RadixVolumeMount
	}
	var tests []struct {
		name   string
		fields fields
		args   args
		want   models.Secret
		want1  models.Secret
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			got, got1 := eh.getBlobFuseSecrets(tt.args.component, tt.args.envNamespace, tt.args.volumeMount)
			assert.Equalf(t, tt.want, got, "getBlobFuseSecrets(%v, %v, %v)", tt.args.component, tt.args.envNamespace, tt.args.volumeMount)
			assert.Equalf(t, tt.want1, got1, "getBlobFuseSecrets(%v, %v, %v)", tt.args.component, tt.args.envNamespace, tt.args.volumeMount)
		})
	}
}

func TestSecretHandler_getCredentialSecretsForBlobVolumes(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		component    v1.RadixCommonDeployComponent
		envNamespace string
	}
	var tests []struct {
		name   string
		fields fields
		args   args
		want   []models.Secret
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			assert.Equalf(t, tt.want, eh.getCredentialSecretsForBlobVolumes(tt.args.component, tt.args.envNamespace), "getCredentialSecretsForBlobVolumes(%v, %v)", tt.args.component, tt.args.envNamespace)
		})
	}
}

func TestSecretHandler_getCredentialSecretsForSecretRefs(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		component    v1.RadixCommonDeployComponent
		envNamespace string
	}
	var tests []struct {
		name    string
		fields  fields
		args    args
		want    []models.Secret
		wantErr assert.ErrorAssertionFunc
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			got, err := eh.getCredentialSecretsForSecretRefs(tt.args.component, tt.args.envNamespace)
			if !tt.wantErr(t, err, fmt.Sprintf("getCredentialSecretsForSecretRefs(%v, %v)", tt.args.component, tt.args.envNamespace)) {
				return
			}
			assert.Equalf(t, tt.want, got, "getCredentialSecretsForSecretRefs(%v, %v)", tt.args.component, tt.args.envNamespace)
		})
	}
}

func TestSecretHandler_getCsiAzureSecrets(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		component    v1.RadixCommonDeployComponent
		envNamespace string
		volumeMount  v1.RadixVolumeMount
	}
	var tests []struct {
		name   string
		fields fields
		args   args
		want   models.Secret
		want1  models.Secret
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			got, got1 := eh.getCsiAzureSecrets(tt.args.component, tt.args.envNamespace, tt.args.volumeMount)
			assert.Equalf(t, tt.want, got, "getCsiAzureSecrets(%v, %v, %v)", tt.args.component, tt.args.envNamespace, tt.args.volumeMount)
			assert.Equalf(t, tt.want1, got1, "getCsiAzureSecrets(%v, %v, %v)", tt.args.component, tt.args.envNamespace, tt.args.volumeMount)
		})
	}
}

func TestSecretHandler_getRadixCommonComponentSecretRefs(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		component    v1.RadixCommonDeployComponent
		envNamespace string
	}
	var tests []struct {
		name    string
		fields  fields
		args    args
		want    []models.Secret
		wantErr assert.ErrorAssertionFunc
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			got, err := eh.getRadixCommonComponentSecretRefs(tt.args.component, tt.args.envNamespace)
			if !tt.wantErr(t, err, fmt.Sprintf("getRadixCommonComponentSecretRefs(%v, %v)", tt.args.component, tt.args.envNamespace)) {
				return
			}
			assert.Equalf(t, tt.want, got, "getRadixCommonComponentSecretRefs(%v, %v)", tt.args.component, tt.args.envNamespace)
		})
	}
}

func TestSecretHandler_getSecretRefsSecrets(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		radixDeployment *v1.RadixDeployment
		envNamespace    string
	}
	var tests []struct {
		name    string
		fields  fields
		args    args
		want    []models.Secret
		wantErr assert.ErrorAssertionFunc
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			got, err := eh.getSecretRefsSecrets(tt.args.radixDeployment, tt.args.envNamespace)
			if !tt.wantErr(t, err, fmt.Sprintf("getSecretRefsSecrets(%v, %v)", tt.args.radixDeployment, tt.args.envNamespace)) {
				return
			}
			assert.Equalf(t, tt.want, got, "getSecretRefsSecrets(%v, %v)", tt.args.radixDeployment, tt.args.envNamespace)
		})
	}
}

func TestSecretHandler_getSecretsForComponent(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		component v1.RadixCommonDeployComponent
	}
	var tests []struct {
		name   string
		fields fields
		args   args
		want   map[string]bool
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			assert.Equalf(t, tt.want, eh.getSecretsForComponent(tt.args.component), "getSecretsForComponent(%v)", tt.args.component)
		})
	}
}

func TestSecretHandler_getSecretsFromAuthentication(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		activeDeployment *v1.RadixDeployment
		envNamespace     string
	}
	var tests []struct {
		name    string
		fields  fields
		args    args
		want    []models.Secret
		wantErr assert.ErrorAssertionFunc
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			got, err := eh.getSecretsFromAuthentication(tt.args.activeDeployment, tt.args.envNamespace)
			if !tt.wantErr(t, err, fmt.Sprintf("getSecretsFromAuthentication(%v, %v)", tt.args.activeDeployment, tt.args.envNamespace)) {
				return
			}
			assert.Equalf(t, tt.want, got, "getSecretsFromAuthentication(%v, %v)", tt.args.activeDeployment, tt.args.envNamespace)
		})
	}
}

func TestSecretHandler_getSecretsFromComponentAuthentication(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		component    v1.RadixCommonDeployComponent
		envNamespace string
	}
	var tests []struct {
		name    string
		fields  fields
		args    args
		want    []models.Secret
		wantErr assert.ErrorAssertionFunc
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			got, err := eh.getSecretsFromComponentAuthentication(tt.args.component, tt.args.envNamespace)
			if !tt.wantErr(t, err, fmt.Sprintf("getSecretsFromComponentAuthentication(%v, %v)", tt.args.component, tt.args.envNamespace)) {
				return
			}
			assert.Equalf(t, tt.want, got, "getSecretsFromComponentAuthentication(%v, %v)", tt.args.component, tt.args.envNamespace)
		})
	}
}

func TestSecretHandler_getSecretsFromComponentAuthenticationClientCertificate(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		component    v1.RadixCommonDeployComponent
		envNamespace string
	}
	var tests []struct {
		name   string
		fields fields
		args   args
		want   []models.Secret
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			assert.Equalf(t, tt.want, eh.getSecretsFromComponentAuthenticationClientCertificate(tt.args.component, tt.args.envNamespace), "getSecretsFromComponentAuthenticationClientCertificate(%v, %v)", tt.args.component, tt.args.envNamespace)
		})
	}
}

func TestSecretHandler_getSecretsFromComponentAuthenticationOAuth2(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		component    v1.RadixCommonDeployComponent
		envNamespace string
	}
	var tests []struct {
		name    string
		fields  fields
		args    args
		want    []models.Secret
		wantErr assert.ErrorAssertionFunc
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			got, err := eh.getSecretsFromComponentAuthenticationOAuth2(tt.args.component, tt.args.envNamespace)
			if !tt.wantErr(t, err, fmt.Sprintf("getSecretsFromComponentAuthenticationOAuth2(%v, %v)", tt.args.component, tt.args.envNamespace)) {
				return
			}
			assert.Equalf(t, tt.want, got, "getSecretsFromComponentAuthenticationOAuth2(%v, %v)", tt.args.component, tt.args.envNamespace)
		})
	}
}

func TestSecretHandler_getSecretsFromLatestDeployment(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		activeDeployment *v1.RadixDeployment
		envNamespace     string
	}
	var tests []struct {
		name    string
		fields  fields
		args    args
		want    map[string]models.Secret
		wantErr assert.ErrorAssertionFunc
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			got, err := eh.getSecretsFromLatestDeployment(tt.args.activeDeployment, tt.args.envNamespace)
			if !tt.wantErr(t, err, fmt.Sprintf("getSecretsFromLatestDeployment(%v, %v)", tt.args.activeDeployment, tt.args.envNamespace)) {
				return
			}
			assert.Equalf(t, tt.want, got, "getSecretsFromLatestDeployment(%v, %v)", tt.args.activeDeployment, tt.args.envNamespace)
		})
	}
}

func TestSecretHandler_getSecretsFromTLSCertificates(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		ra           *v1.RadixApplication
		envName      string
		envNamespace string
	}
	var tests []struct {
		name    string
		fields  fields
		args    args
		want    map[string]models.Secret
		wantErr assert.ErrorAssertionFunc
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			got, err := eh.getSecretsFromTLSCertificates(tt.args.ra, tt.args.envName, tt.args.envNamespace)
			if !tt.wantErr(t, err, fmt.Sprintf("getSecretsFromTLSCertificates(%v, %v, %v)", tt.args.ra, tt.args.envName, tt.args.envNamespace)) {
				return
			}
			assert.Equalf(t, tt.want, got, "getSecretsFromTLSCertificates(%v, %v, %v)", tt.args.ra, tt.args.envName, tt.args.envNamespace)
		})
	}
}

func TestSecretHandler_getSecretsFromVolumeMounts(t *testing.T) {
	type fields struct {
		client          kubernetes.Interface
		radixclient     versioned.Interface
		inClusterClient kubernetes.Interface
		deployHandler   deployments.DeployHandler
		eventHandler    events.EventHandler
		accounts        apiModels.Accounts
		kubeUtil        *kube.Kube
	}
	type args struct {
		activeDeployment *v1.RadixDeployment
		envNamespace     string
	}
	var tests []struct {
		name    string
		fields  fields
		args    args
		want    []models.Secret
		wantErr assert.ErrorAssertionFunc
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eh := SecretHandler{
				client:          tt.fields.client,
				radixclient:     tt.fields.radixclient,
				inClusterClient: tt.fields.inClusterClient,
				deployHandler:   tt.fields.deployHandler,
				eventHandler:    tt.fields.eventHandler,
				accounts:        tt.fields.accounts,
				kubeUtil:        tt.fields.kubeUtil,
			}
			got, err := eh.getSecretsFromVolumeMounts(tt.args.activeDeployment, tt.args.envNamespace)
			if !tt.wantErr(t, err, fmt.Sprintf("getSecretsFromVolumeMounts(%v, %v)", tt.args.activeDeployment, tt.args.envNamespace)) {
				return
			}
			assert.Equalf(t, tt.want, got, "getSecretsFromVolumeMounts(%v, %v)", tt.args.activeDeployment, tt.args.envNamespace)
		})
	}
}

func TestWithAccounts(t *testing.T) {
	type args struct {
		accounts apiModels.Accounts
	}
	var tests []struct {
		name string
		args args
		want SecretHandlerOptions
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, WithAccounts(tt.args.accounts), "WithAccounts(%v)", tt.args.accounts)
		})
	}
}

func TestWithEventHandler(t *testing.T) {
	type args struct {
		eventHandler events.EventHandler
	}
	var tests []struct {
		name string
		args args
		want SecretHandlerOptions
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, WithEventHandler(tt.args.eventHandler), "WithEventHandler(%v)", tt.args.eventHandler)
		})
	}
}
