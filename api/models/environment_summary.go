package models

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	environmentModels "github.com/equinor/radix-api/api/environments/models"
	"github.com/equinor/radix-api/api/utils/predicate"
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// BuildEnvironmentSummaryList builds a list of EnvironmentSummary models.
func BuildEnvironmentSummaryList(rr *radixv1.RadixRegistration, ra *radixv1.RadixApplication, reList []radixv1.RadixEnvironment, rdList []radixv1.RadixDeployment, rjList []radixv1.RadixJob) []*environmentModels.EnvironmentSummary {
	var envList []*environmentModels.EnvironmentSummary

	getActiveDeploymentSummary := func(appName, envName string, rds []radixv1.RadixDeployment) *deploymentModels.DeploymentSummary {
		if activeRd, ok := GetActiveDeploymentForAppEnv(appName, envName, rds); ok {
			return BuildDeploymentSummary(&activeRd, rr, rjList)
		}
		return nil
	}

	for _, e := range ra.Spec.Environments {
		var re *radixv1.RadixEnvironment
		if foundRe, ok := slice.FindFirst(reList, func(re radixv1.RadixEnvironment) bool { return re.Spec.AppName == ra.Name && re.Spec.EnvName == e.Name }); ok {
			re = &foundRe
		}

		env := &environmentModels.EnvironmentSummary{
			Name:             e.Name,
			BranchMapping:    e.Build.From,
			ActiveDeployment: getActiveDeploymentSummary(ra.GetName(), e.Name, rdList),
			Status:           getEnvironmentConfigurationStatus(re).String(),
		}
		envList = append(envList, env)
	}

	for _, re := range slice.FindAll(reList, predicate.IsOrphanEnvironment) {
		env := &environmentModels.EnvironmentSummary{
			Name:             re.Spec.EnvName,
			ActiveDeployment: getActiveDeploymentSummary(ra.GetName(), re.Spec.EnvName, rdList),
			Status:           getEnvironmentConfigurationStatus(&re).String(),
		}
		envList = append(envList, env)
	}

	return envList
}
