package privateimagehubs

import (
	"context"

	"github.com/equinor/radix-api/api/privateimagehubs/models"
	"github.com/equinor/radix-api/api/utils"
	sharedModels "github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
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
	pendingImageHubSecrets, err := applicationconfig.GetPendingPrivateImageHubSecrets(ph.kubeUtil, appName)
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
	return applicationconfig.UpdatePrivateImageHubsSecretsPassword(ph.kubeUtil, appName, server, password)
}

func getImageHubSecretStatus(pendingImageHubSecrets []string, server string) models.ImageHubSecretStatus {
	for _, val := range pendingImageHubSecrets {
		if val == server {
			return models.Pending
		}
	}
	return models.Consistent
}
