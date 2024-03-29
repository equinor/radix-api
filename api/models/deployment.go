package models

import (
	deploymentModels "github.com/equinor/radix-api/api/deployments/models"
	"github.com/equinor/radix-api/api/utils/tlsvalidation"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
)

// BuildDeployment builds a Deployment model.
func BuildDeployment(rr *radixv1.RadixRegistration, ra *radixv1.RadixApplication, rd *radixv1.RadixDeployment, deploymentList []appsv1.Deployment, podList []corev1.Pod, hpaList []autoscalingv2.HorizontalPodAutoscaler, secretList []corev1.Secret, tlsValidator tlsvalidation.Validator, rjList []radixv1.RadixJob) *deploymentModels.Deployment {
	components := BuildComponents(ra, rd, deploymentList, podList, hpaList, secretList, tlsValidator)

	// The only error that can be returned from DeploymentBuilder is related to errors from github.com/imdario/mergo
	// This type of error will only happen if incorrect objects (e.g. incompatible structs) are sent as arguments to mergo,
	// and we should consider to panic the error in the code calling merge.
	// For now we will panic the error here.
	radixJob, _ := slice.FindFirst(rjList, func(radixJob radixv1.RadixJob) bool {
		return radixJob.GetName() == rd.GetLabels()[kube.RadixJobNameLabel]
	})
	deployment, err := deploymentModels.NewDeploymentBuilder().
		WithRadixRegistration(rr).
		WithRadixDeployment(rd).
		WithPipelineJob(&radixJob).
		WithGitCommitHash(rd.Annotations[kube.RadixCommitAnnotation]).
		WithGitTags(rd.Annotations[kube.RadixGitTagsAnnotation]).
		WithComponents(components).
		BuildDeployment()
	if err != nil {
		panic(err)
	}

	return deployment

}
