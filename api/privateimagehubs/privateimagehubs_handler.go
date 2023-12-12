package privateimagehubs

import (
	"context"
	"fmt"

	"github.com/equinor/radix-api/api/privateimagehubs/models"
	"github.com/equinor/radix-api/api/utils"
	sharedModels "github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// PrivateImageHubHandler Instance variables
type PrivateImageHubHandler struct {
	userAccount    sharedModels.Account
	serviceAccount sharedModels.Account
	kubeUtil       *kube.Kube
}

// Init Constructor
func Init(accounts sharedModels.Accounts) PrivateImageHubHandler {
	kubeUtil, _ := kube.New(accounts.UserAccount.Client, accounts.UserAccount.RadixClient, accounts.UserAccount.SecretProviderClient)
	return PrivateImageHubHandler{
		userAccount:    accounts.UserAccount,
		serviceAccount: accounts.ServiceAccount,
		kubeUtil:       kubeUtil,
	}
}

// GetPrivateImageHubs returns all private image hubs defined for app
func (ph PrivateImageHubHandler) GetPrivateImageHubs(ctx context.Context, appName string) ([]models.ImageHubSecret, error) {
	var imageHubSecrets []models.ImageHubSecret
	application, err := utils.CreateApplicationConfig(ctx, &ph.userAccount, appName)
	if err != nil {
		return []models.ImageHubSecret{}, nil
	}
	pendingImageHubSecrets, err := GetPendingPrivateImageHubSecrets(ph.kubeUtil, appName)
	if err != nil {
		return nil, err
	}

	radixApp := application.GetRadixApplicationConfig()
	for server, config := range radixApp.Spec.PrivateImageHubs {
		imageHubSecrets = append(imageHubSecrets, models.ImageHubSecret{
			Server:   server,
			Username: config.Username,
			Email:    config.Email,
			Status:   getImageHubSecretStatus(pendingImageHubSecrets, server).String(),
		})
	}

	return imageHubSecrets, nil
}

// UpdatePrivateImageHubValue updates the private image hub value with new password
func (ph PrivateImageHubHandler) UpdatePrivateImageHubValue(appName, server, password string) error {
	namespace := operatorutils.GetAppNamespace(appName)
	secret, _ := ph.kubeUtil.GetSecret(namespace, defaults.PrivateImageHubSecretName)
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
		return applicationconfig.ApplyPrivateImageHubSecret(ph.kubeUtil, namespace, appName, secretValue)
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

func getImageHubSecretStatus(pendingImageHubSecrets []string, server string) models.ImageHubSecretStatus {
	for _, val := range pendingImageHubSecrets {
		if val == server {
			return models.Pending
		}
	}
	return models.Consistent
}
