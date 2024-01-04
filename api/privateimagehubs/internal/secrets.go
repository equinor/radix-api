package internal

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// UpdatePrivateImageHubsSecretsPassword update secret password
func UpdatePrivateImageHubsSecretsPassword(kubeUtil *kube.Kube, appName, server, password string) error {
	namespace := operatorutils.GetAppNamespace(appName)
	secret, _ := kubeUtil.GetSecret(namespace, defaults.PrivateImageHubSecretName)
	if secret == nil {
		return fmt.Errorf("private image hub secret does not exist for app %s", appName)
	}

	imageHubs, err := applicationconfig.GetImageHubSecretValue(secret.Data[corev1.DockerConfigJsonKey])
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
		return applicationconfig.ApplyPrivateImageHubSecret(kubeUtil, namespace, appName, secretValue)
	}
	return fmt.Errorf("private image hub secret does not contain config for server %s", server)
}

// GetPendingPrivateImageHubSecrets returns a list of private image hubs where secret value is not set
func GetPendingPrivateImageHubSecrets(kubeUtil *kube.Kube, appName string) ([]string, error) {
	pendingSecrets := []string{}
	ns := operatorutils.GetAppNamespace(appName)
	secret, err := kubeUtil.GetSecret(ns, defaults.PrivateImageHubSecretName)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

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
