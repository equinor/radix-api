package applications

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"

	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	operatornumbers "github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
)

// IsDeployKeyValid Checks if deploy key for app is correctly setup
func IsDeployKeyValid(ctx context.Context, account models.Account, appName string) (bool, error) {
	rr, err := account.RadixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	if rr.Spec.CloneURL == "" {
		return false, radixhttp.ValidationError("Radix Registration", "Clone URL is missing")
	}

	if rr.Spec.DeployKey == "" {
		return false, radixhttp.ValidationError("Radix Registration", "Deploy key is missing")
	}

	err = verifyDeployKey(ctx, account.Client, rr)
	return err == nil, err
}

func verifyDeployKey(ctx context.Context, client kubernetes.Interface, rr *v1.RadixRegistration) error {
	namespace := utils.GetAppNamespace(rr.Name)
	jobApplied, err := createCloneJob(ctx, client, rr)
	if err != nil {
		return err
	}

	w, err := client.BatchV1().Jobs(jobApplied.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fields.Set{"metadata.name": jobApplied.Name}.AsSelector().String(),
	})
	if err != nil {
		return err
	}
	defer w.Stop()
	defer cleanup(ctx, client, namespace, jobApplied)

	for events := range w.ResultChan() {
		j, ok := events.Object.(*batchv1.Job)
		switch {
		case ok && j.Status.Succeeded > 0:
			return nil
		case ok && j.Status.Failed > 0:
			message := "Deploy key was invalid"
			if isJobStatusFailedWithDeadlineExceeded(j) {
				message = "Deploy key validation timed out"
			}
			return radixhttp.ValidationError("Radix Registration", message)
		default:

			log.Ctx(ctx).Debug().Msg("Ongoing - build docker image")
		}
	}

	return errors.New("channel failed")
}

func isJobStatusFailedWithDeadlineExceeded(job *batchv1.Job) bool {
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue && cond.Reason == "DeadlineExceeded" {
			return true
		}
	}

	return false
}

func createCloneJob(ctx context.Context, client kubernetes.Interface, rr *v1.RadixRegistration) (*batchv1.Job, error) {
	jobName := strings.ToLower(fmt.Sprintf("%s-%s", rr.Name, radixutils.RandString(5)))
	namespace := utils.GetAppNamespace(rr.Name)
	backOffLimit := int32(0)
	deadlineSeconds := operatornumbers.Int64Ptr(5 * 60)
	defaultMode := int32(256)
	initContainers := git.CloneInitContainers(rr.Spec.CloneURL, applicationconfig.GetConfigBranch(rr))

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				kube.RadixJobNameLabel: jobName,
				kube.RadixJobTypeLabel: "validate-clone-url",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backOffLimit,
			ActiveDeadlineSeconds: deadlineSeconds,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "radix-pipeline",
					Containers:         initContainers,
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: operatornumbers.Int64Ptr(1000),
					},
					Volumes: []corev1.Volume{
						{
							Name: git.BuildContextVolumeName,
						},
						{
							Name: git.GitSSHKeyVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  git.GitSSHKeyVolumeName,
									DefaultMode: &defaultMode,
								},
							},
						},
					},
					RestartPolicy: "Never",
				},
			},
		},
	}

	return client.BatchV1().Jobs(namespace).Create(ctx, &job, metav1.CreateOptions{})
}

func cleanup(ctx context.Context, client kubernetes.Interface, namespace string, jobApplied *batchv1.Job) {
	_ = client.BatchV1().Jobs(namespace).Delete(ctx, jobApplied.Name, metav1.DeleteOptions{})
}
