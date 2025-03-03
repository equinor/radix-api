package internal

import (
	"context"
	"fmt"
	"time"

	"github.com/equinor/radix-api/api/kubequery"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
)

// UpdatePrivateImageHubsSecretsPassword update secret password
func UpdatePrivateImageHubsSecretsPassword(ctx context.Context, kubeUtil *kube.Kube, appName, server, password string) error {
	namespace := operatorutils.GetAppNamespace(appName)
	original, _ := kubeUtil.GetSecret(ctx, namespace, defaults.PrivateImageHubSecretName)
	if original == nil {
		return fmt.Errorf("private image hub secret does not exist for app %s", appName)
	}

	modified := original.DeepCopy()

	imageHubs, err := applicationconfig.GetImageHubSecretValue(modified.Data[corev1.DockerConfigJsonKey])
	if err != nil {
		return err
	}

	if config, ok := imageHubs[server]; ok {
		config.Password = password
		imageHubs[server] = config
		secretValue, err := applicationconfig.GetImageHubsSecretValue(imageHubs)
		if err != nil {
			return err
		}

		modified.Data[corev1.DockerConfigJsonKey] = secretValue

		if err = kubequery.PatchSecretMetadata(modified, server, time.Now()); err != nil {
			return err
		}

		_, err = kubeUtil.UpdateSecret(ctx, original, modified)
		return err
	}
	return fmt.Errorf("private image hub secret does not contain config for server %s", server)
}

// GetPendingPrivateImageHubSecrets returns a list of private image hubs where secret value is not set
func GetPendingPrivateImageHubSecrets(secret *corev1.Secret) ([]string, error) {
	var pendingSecrets []string
	imageHubs, err := applicationconfig.GetImageHubSecretValue(secret.Data[corev1.DockerConfigJsonKey])
	if err != nil {
		return nil, err
	}

	for key, imageHub := range imageHubs {
		if imageHub.Password == "" {
			pendingSecrets = append(pendingSecrets, key)
		}
	}
	return pendingSecrets, nil
}
