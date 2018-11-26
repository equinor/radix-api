package deployments

import (
	"strings"

	log "github.com/Sirupsen/logrus"
	deploymentModels "github.com/statoil/radix-api/api/deployments/models"
	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"github.com/statoil/radix-operator/pkg/apis/radixvalidators"
	crdUtils "github.com/statoil/radix-operator/pkg/apis/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HandlePromoteToEnvironment handler for PromoteEnvironment
func (deploy DeployHandler) HandlePromoteToEnvironment(appName, deploymentName string, promotionParameters deploymentModels.PromotionParameters) (*deploymentModels.DeploymentSummary, error) {
	if strings.TrimSpace(appName) == "" {
		return nil, utils.ValidationError("Radix Promotion", "App name is required")
	}

	radixConfig, err := deploy.radixClient.RadixV1().RadixApplications(crdUtils.GetAppNamespace(appName)).Get(appName, metav1.GetOptions{})
	if err != nil {
		return nil, nonExistingApplication(err, appName)
	}

	fromNs := crdUtils.GetEnvironmentNamespace(appName, promotionParameters.FromEnvironment)
	toNs := crdUtils.GetEnvironmentNamespace(appName, promotionParameters.ToEnvironment)

	_, err = deploy.kubeClient.CoreV1().Namespaces().Get(fromNs, metav1.GetOptions{})
	if err != nil {
		return nil, nonExistingFromEnvironment(err)
	}

	_, err = deploy.kubeClient.CoreV1().Namespaces().Get(toNs, metav1.GetOptions{})
	if err != nil {
		return nil, nonExistingToEnvironment(err)
	}

	log.Infof("Promoting %s from %s to %s", appName, promotionParameters.FromEnvironment, promotionParameters.ToEnvironment)
	var radixDeployment *v1.RadixDeployment

	radixDeployment, err = deploy.radixClient.RadixV1().RadixDeployments(fromNs).Get(deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, nonExistingDeployment(err, deploymentName)
	}

	radixDeployment.ResourceVersion = ""
	radixDeployment.Namespace = toNs
	radixDeployment.Spec.Environment = promotionParameters.ToEnvironment

	err = mergeWithRadixApplication(radixConfig, radixDeployment, promotionParameters.ToEnvironment)
	if err != nil {
		return nil, err
	}

	isValid, err := radixvalidators.CanRadixDeploymentBeInserted(deploy.radixClient, radixDeployment)
	if !isValid {
		return nil, err
	}

	radixDeployment, err = deploy.radixClient.RadixV1().RadixDeployments(toNs).Create(radixDeployment)
	if err != nil {
		return nil, err
	}

	return &deploymentModels.DeploymentSummary{Name: radixDeployment.Name}, nil
}

func mergeWithRadixApplication(radixConfig *v1.RadixApplication, radixDeployment *v1.RadixDeployment, environment string) error {
	for index, comp := range radixDeployment.Spec.Components {
		raComp := getComponentConfig(radixConfig, comp.Name)
		if raComp == nil {
			return nonExistingComponentName(radixConfig.GetName(), comp.Name)
		}

		environmentVariables := getEnvironmentVariables(raComp, environment)
		radixDeployment.Spec.Components[index].EnvironmentVariables = environmentVariables
	}

	return nil
}

func getComponentConfig(radixConfig *v1.RadixApplication, componentName string) *v1.RadixComponent {
	for _, comp := range radixConfig.Spec.Components {
		if strings.EqualFold(comp.Name, componentName) {
			return &comp
		}
	}

	return nil
}

func getEnvironmentVariables(componentConfig *v1.RadixComponent, environment string) v1.EnvVarsMap {
	for _, environmentVariables := range componentConfig.EnvironmentVariables {
		if strings.EqualFold(environmentVariables.Environment, environment) {
			return environmentVariables.Variables
		}
	}

	return v1.EnvVarsMap{}
}
