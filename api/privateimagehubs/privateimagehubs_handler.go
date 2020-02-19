package privateimagehubs

import (
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
func (ph PrivateImageHubHandler) GetPrivateImageHubs(appName string) ([]models.ImageHubSecret, error) {
	imageHubSecrets := []models.ImageHubSecret{}
	application, err := utils.CreateApplicationConfig(ph.userAccount.Client, ph.userAccount.RadixClient, appName)
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
			Status:   getImageHubSecretStatus(pendingImageHubSecrets, server),
		})
	}

	return imageHubSecrets, nil
}

// UpdatePrivateImageHubValue updates the private image hub value with new password
func (ph PrivateImageHubHandler) UpdatePrivateImageHubValue(appName, server, password string) error {
	application, err := utils.CreateApplicationConfig(ph.userAccount.Client, ph.userAccount.RadixClient, appName)
	if err != nil {
		return err
	}
	return application.UpdatePrivateImageHubsSecretsPassword(server, password)
}

func getImageHubSecretStatus(pendingImageHubSecrets []string, server string) string {
	for _, val := range pendingImageHubSecrets {
		if val == server {
			return "Pending"
		}
	}

	return "Consistent"
}
