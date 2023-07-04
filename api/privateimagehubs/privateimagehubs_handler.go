package privateimagehubs

import (
	"context"
	"fmt"

	"github.com/equinor/radix-api/api/privateimagehubs/models"
	"github.com/equinor/radix-api/api/utils"
	sharedModels "github.com/equinor/radix-api/models"
)

// PrivateImageHubHandler Instance variables
type PrivateImageHubHandler struct {
	userAccount    sharedModels.Account
	serviceAccount sharedModels.Account
}

// Init Constructor
func Init(accounts sharedModels.Accounts) PrivateImageHubHandler {

	return PrivateImageHubHandler{
		userAccount:    accounts.UserAccount,
		serviceAccount: accounts.ServiceAccount}
}

// GetPrivateImageHubs returns all private image hubs defined for app
func (ph PrivateImageHubHandler) GetPrivateImageHubs(ctx context.Context, appName string) ([]models.ImageHubSecret, error) {
	var imageHubSecrets []models.ImageHubSecret
	application, err := utils.CreateApplicationConfig(ctx, &ph.userAccount, appName)
	if err != nil {
		return []models.ImageHubSecret{}, nil
	}
	pendingImageHubSecrets, err := application.GetPendingPrivateImageHubSecrets()
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
func (ph PrivateImageHubHandler) UpdatePrivateImageHubValue(ctx context.Context, appName, server, password string) error {

	userIsAdmin, err := utils.UserIsAdmin(ctx, &ph.userAccount, appName)
	if err != nil {
		return err
	}
	if !userIsAdmin {
		return fmt.Errorf("user is not allowed to update private image hubs for %s", appName)
	}
	application, err := utils.CreateApplicationConfig(ctx, &ph.userAccount, appName)
	if err != nil {
		return err
	}
	return application.UpdatePrivateImageHubsSecretsPassword(server, password)
}

func getImageHubSecretStatus(pendingImageHubSecrets []string, server string) models.ImageHubSecretStatus {
	for _, val := range pendingImageHubSecrets {
		if val == server {
			return models.Pending
		}
	}
	return models.Consistent
}
