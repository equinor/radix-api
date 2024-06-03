package utils

import (
	"context"
	"strings"

	"github.com/equinor/radix-api/models"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

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

	registration, err := user.RadixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	kubeUtils, _ := kube.New(user.Client, user.RadixClient, user.KedaClient, user.SecretProviderClient)
	return applicationconfig.NewApplicationConfig(user.Client, kubeUtils, user.RadixClient, registration, radixApp, &dnsalias.DNSConfig{}), nil
}

// GetComponentEnvironmentConfig Gets environment config of component
func GetComponentEnvironmentConfig(ra *radixv1.RadixApplication, envName, componentName string) radixv1.RadixCommonEnvironmentConfig {
	component := getRadixCommonComponentByName(ra, componentName)
	if component == nil {
		return nil
	}
	return getEnvironmentConfigByName(component, envName)
}

func getEnvironmentConfigByName(component radixv1.RadixCommonComponent, envName string) radixv1.RadixCommonEnvironmentConfig {
	for _, environment := range component.GetEnvironmentConfig() {
		if strings.EqualFold(environment.GetEnvironment(), envName) {
			return environment
		}
	}
	return nil
}

func getRadixCommonComponentByName(ra *radixv1.RadixApplication, name string) radixv1.RadixCommonComponent {
	for _, component := range ra.Spec.Components {
		if strings.EqualFold(component.Name, name) {
			return &component
		}
	}
	for _, jobComponent := range ra.Spec.Jobs {
		if strings.EqualFold(jobComponent.Name, name) {
			return &jobComponent
		}
	}
	return nil
}
