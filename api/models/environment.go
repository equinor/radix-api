package models

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	secretModels "github.com/equinor/radix-api/api/secrets/models"
	"github.com/equinor/radix-api/api/utils/tlsvalidator"
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
)

func BuildEnvironment(rr *radixv1.RadixRegistration, ra *radixv1.RadixApplication, re *radixv1.RadixEnvironment, rdList []radixv1.RadixDeployment, rjList []radixv1.RadixJob, deploymentList []appsv1.Deployment, podList []corev1.Pod, hpaList []autoscalingv2.HorizontalPodAutoscaler, secretList []corev1.Secret, tlsValidator tlsvalidator.Interface) *environmentModels.Environment {
	var buildFromBranch string
	var activeDeployment *deploymentModels.Deployment
	var secrets []secretModels.Secret

	if raEnv := getRadixApplicationEnvironment(ra, re.Spec.EnvName); raEnv != nil {
		buildFromBranch = raEnv.Build.From
	}

	if i := slice.FindIndex(rdList, isActiveDeploymentForAppAndEnv(ra.Name, re.Spec.EnvName)); i >= 0 {
		activeRd := &rdList[i]
		activeDeployment = BuildDeployment(rr, ra, activeRd, deploymentList, podList, hpaList)
		secrets = BuildSecrets(secretList, activeRd, tlsValidator)
	}

	return &environmentModels.Environment{
		Name:             re.Spec.EnvName,
		BranchMapping:    buildFromBranch,
		Status:           getEnvironmentConfigurationStatus(re).String(),
		Deployments:      BuildDeploymentSummaryList(rr, rdList, rjList),
		ActiveDeployment: activeDeployment,
		Secrets:          secrets,
	}
}

func getRadixApplicationEnvironment(ra *radixv1.RadixApplication, envName string) *radixv1.Environment {
	envIdx := slice.FindIndex(ra.Spec.Environments, func(env radixv1.Environment) bool {
		return env.Name == envName
	})
	if envIdx >= 0 {
		return &ra.Spec.Environments[envIdx]
	}
	return nil
}
