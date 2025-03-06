package privateimagehubs

import (
	"context"

	"github.com/equinor/radix-api/api/kubequery"
	"github.com/equinor/radix-api/api/privateimagehubs/internal"
	"github.com/equinor/radix-api/api/privateimagehubs/models"
	"github.com/equinor/radix-api/api/utils"
	sharedModels "github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
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
	kubeUtil, _ := kube.New(accounts.UserAccount.Client, accounts.UserAccount.RadixClient, accounts.UserAccount.KedaClient, accounts.UserAccount.SecretProviderClient)
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

	ns := operatorutils.GetAppNamespace(appName)
	secret, err := ph.kubeUtil.GetSecret(ctx, ns, defaults.PrivateImageHubSecretName)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	pendingImageHubSecrets, err := internal.GetPendingPrivateImageHubSecrets(secret)
	if err != nil {
		return nil, err
	}

	metadata := kubequery.GetSecretMetadata(ctx, secret)
	radixApp := application.GetRadixApplicationConfig()
	for server, config := range radixApp.Spec.PrivateImageHubs {
		imageHubSecrets = append(imageHubSecrets, models.ImageHubSecret{
			Server:   server,
			Username: config.Username,
			Email:    config.Email,
			Status:   getImageHubSecretStatus(pendingImageHubSecrets, server).String(),
			Updated:  metadata.GetUpdated(server),
		})
	}

	return imageHubSecrets, nil
}

// UpdatePrivateImageHubValue updates the private image hub value with new password
func (ph PrivateImageHubHandler) UpdatePrivateImageHubValue(ctx context.Context, appName, server, password string) error {
	return internal.UpdatePrivateImageHubsSecretsPassword(ctx, ph.kubeUtil, appName, server, password)
}

func getImageHubSecretStatus(pendingImageHubSecrets []string, server string) models.ImageHubSecretStatus {
	for _, val := range pendingImageHubSecrets {
		if val == server {
			return models.Pending
		}
	}
	return models.Consistent
}
