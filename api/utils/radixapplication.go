package utils

import (
	"context"
	"fmt"
	"github.com/equinor/radix-api/models"

	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateApplicationConfig creates an application config based on input
func CreateApplicationConfig(ctx context.Context, user *models.Account, appName string) (*applicationconfig.ApplicationConfig, error) {
	radixApp, err := user.RadixClient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// review comment: this block adds an extra request to the API server
	userIsAdmin, err := UserIsAdmin(ctx, user, appName)
	if err != nil {
		return nil, err
	}
	if !userIsAdmin {
		return nil, fmt.Errorf("user is not allowed to create application config for %s", appName)
	}

	registration, err := user.RadixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	kubeUtils, _ := kube.New(user.Client, user.RadixClient, user.SecretProviderClient)
	return applicationconfig.NewApplicationConfig(user.Client, kubeUtils, user.RadixClient, registration, radixApp)
}
