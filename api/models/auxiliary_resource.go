package models

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/utils/predicate"
	"github.com/equinor/radix-common/utils/slice"
	operatordefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func getAuxiliaryResources(appName string, component radixv1.RadixCommonDeployComponent, deploymentList []appsv1.Deployment, podList []corev1.Pod, eventWarnings map[string]string) deploymentModels.AuxiliaryResource {
	var auxResource deploymentModels.AuxiliaryResource
	if auth := component.GetAuthentication(); component.IsPublic() && auth != nil && auth.OAuth2 != nil {
		auxResource.OAuth2 = getOAuth2AuxiliaryResource(appName, component.GetName(), deploymentList, podList, eventWarnings)
	}
	return auxResource
}

func getOAuth2AuxiliaryResource(appName, componentName string, deploymentList []appsv1.Deployment, podList []corev1.Pod, eventWarnings map[string]string) *deploymentModels.OAuth2AuxiliaryResource {
	return &deploymentModels.OAuth2AuxiliaryResource{
		Deployment: getAuxiliaryResourceDeployment(appName, componentName, operatordefaults.OAuthProxyAuxiliaryComponentType, deploymentList, podList, eventWarnings),
	}
}

func getAuxiliaryResourceDeployment(appName, componentName, auxType string, deploymentList []appsv1.Deployment, podList []corev1.Pod, eventWarnings map[string]string) deploymentModels.AuxiliaryResourceDeployment {
	var auxResourceDeployment deploymentModels.AuxiliaryResourceDeployment
	auxDeployments := slice.FindAll(deploymentList, predicate.IsDeploymentForAuxComponent(appName, componentName, auxType))
	if len(auxDeployments) == 0 {
		auxResourceDeployment.Status = deploymentModels.ComponentReconciling.String()
		return auxResourceDeployment
	}
	deployment := auxDeployments[0]
	auxPods := slice.FindAll(podList, predicate.IsPodForAuxComponent(appName, componentName, auxType))
	auxResourceDeployment.ReplicaList = BuildReplicaSummaryList(auxPods, eventWarnings)
	auxResourceDeployment.Status = deploymentModels.ComponentStatusFromDeployment(&deployment).String()
	return auxResourceDeployment
}
