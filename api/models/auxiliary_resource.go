package models

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/utils/predicate"
	"github.com/equinor/radix-common/utils/slice"
	operatordefaults "github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func getAuxiliaryResources(rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, deploymentList []appsv1.Deployment, podList []corev1.Pod, eventWarnings map[string]string) deploymentModels.AuxiliaryResource {
	var auxResource deploymentModels.AuxiliaryResource
	if auth := component.GetAuthentication(); component.IsPublic() && auth != nil && auth.OAuth2 != nil {
		auxResource.OAuth2 = getOAuth2AuxiliaryResource(rd, component, deploymentList, podList, eventWarnings)
	}
	return auxResource
}

func getOAuth2AuxiliaryResource(rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, deploymentList []appsv1.Deployment, podList []corev1.Pod, eventWarnings map[string]string) *deploymentModels.OAuth2AuxiliaryResource {
	auxiliaryResource := deploymentModels.OAuth2AuxiliaryResource{
		Deployment: getAuxiliaryResourceDeployment(rd, component, operatordefaults.OAuthProxyAuxiliaryComponentType, deploymentList, podList, eventWarnings),
	}
	oauth2 := component.GetAuthentication().GetOAuth2()
	if oauth2.GetUseAzureIdentity() {
		auxiliaryResource.Identity = &deploymentModels.Identity{
			Azure: &deploymentModels.AzureIdentity{
				ClientId:           oauth2.ClientID,
				ServiceAccountName: utils.GetAuxOAuthServiceAccountName(component.GetName()),
			},
		}
	}
	return &auxiliaryResource
}

func getAuxiliaryResourceDeployment(rd *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, auxType string, deploymentList []appsv1.Deployment, podList []corev1.Pod, eventWarnings map[string]string) deploymentModels.AuxiliaryResourceDeployment {
	var auxResourceDeployment deploymentModels.AuxiliaryResourceDeployment
	auxDeployments := slice.FindAll(deploymentList, predicate.IsDeploymentForAuxComponent(rd.Spec.AppName, component.GetName(), auxType))
	if len(auxDeployments) == 0 {
		auxResourceDeployment.Status = deploymentModels.ComponentReconciling.String()
		return auxResourceDeployment
	}
	deployment := auxDeployments[0]
	auxPods := slice.FindAll(podList, predicate.IsPodForAuxComponent(rd.Spec.AppName, component.GetName(), auxType))
	auxResourceDeployment.ReplicaList = BuildReplicaSummaryList(auxPods, eventWarnings)
	auxResourceDeployment.Status = deploymentModels.ComponentStatusFromDeployment(component, &deployment, rd).String()
	return auxResourceDeployment
}
