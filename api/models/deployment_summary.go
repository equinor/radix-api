package models

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

func BuildDeploymentSummary(rd *radixv1.RadixDeployment, rr *radixv1.RadixRegistration, rjList []radixv1.RadixJob) *deploymentModels.DeploymentSummary {
	var rj *radixv1.RadixJob
	if i := slice.FindIndex(rjList, func(rj radixv1.RadixJob) bool { return rd.Labels[kube.RadixJobNameLabel] == rj.Name }); i >= 0 {
		rj = &rjList[i]
	}

	deploymentSummary, err := deploymentModels.
		NewDeploymentBuilder().
		WithRadixDeployment(rd).
		WithPipelineJob(rj).
		WithRadixRegistration(rr).
		BuildDeploymentSummary()

	// The only error that can be returned from DeploymentBuilder is related to errors from github.com/imdario/mergo
	// This type of error will only happen if incorrect objects (e.g. incompatible structs) are sent as arguments to mergo,
	// and we should consider to panic the error in the code calling merge.
	// For now we will panic the error here.
	if err != nil {
		panic(err)
	}
	return deploymentSummary
}
