package applications

import (
	"context"
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"strings"

	"github.com/equinor/radix-api/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	operatornumbers "github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
)

const containerRegistryEnvironmentVariable = "RADIX_CONTAINER_REGISTRY"

// IsDeployKeyValid Checks if deploy key for app is correctly setup
func IsDeployKeyValid(account models.Account, appName string) (bool, error) {
	rr, err := account.RadixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	if rr.Spec.CloneURL == "" {
		return false, radixhttp.ValidationError("Radix Registration", fmt.Sprintf("Clone URL is missing"))
	}

	if rr.Spec.DeployKey == "" {
		return false, radixhttp.ValidationError("Radix Registration", fmt.Sprintf("Deploy key is missing"))
	}

	err = verifyDeployKey(account.Client, rr)
	return err == nil, err
}

func verifyDeployKey(client kubernetes.Interface, rr *v1.RadixRegistration) error {
	namespace := utils.GetAppNamespace(rr.Name)
	jobApplied, err := createCloneJob(client, rr)

	w, err := client.BatchV1().Jobs(jobApplied.Namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: fields.Set{"metadata.name": jobApplied.Name}.AsSelector().String(),
	})
	if err != nil {
		return err
	}
	done := make(chan error)
	go func() {
		for {
			select {
			case events, ok := <-w.ResultChan():
				if !ok {
					done <- fmt.Errorf("Channel failed")
				}

				j := events.Object.(*batchv1.Job)
				switch {
				case j.Status.Succeeded == 1:
					cleanup(client, namespace, jobApplied)
					done <- nil
					return
				case j.Status.Failed == 1:
					cleanup(client, namespace, jobApplied)
					done <- radixhttp.ValidationError("Radix Registration", fmt.Sprintf("Deploy key was invalid"))
					return
				default:
					log.Debugf("Ongoing - build docker image")
				}
			}
		}
	}()

	err = <-done
	return err
}

func createCloneJob(client kubernetes.Interface, rr *v1.RadixRegistration) (*batchv1.Job, error) {
	jobName := strings.ToLower(fmt.Sprintf("%s-%s", rr.Name, radixutils.RandString(5)))
	namespace := utils.GetAppNamespace(rr.Name)
	backOffLimit := int32(1)

	defaultMode := int32(256)
	privileged, allowPrivilegeEscalation := false, false
	securityContext := corev1.SecurityContext{
		Privileged:               &privileged,
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		RunAsUser:                operatornumbers.Int64Ptr(1000),
		RunAsGroup:               operatornumbers.Int64Ptr(1000),
	}
	initContainers := git.CloneInitContainers(rr.Spec.CloneURL, applicationconfig.GetConfigBranch(rr), securityContext, "")

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				kube.RadixJobNameLabel: jobName,
				kube.RadixJobTypeLabel: "validate-clone-url",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "radix-pipeline",
					Containers:         initContainers,
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

	jobApplied, err := client.BatchV1().Jobs(namespace).Create(context.TODO(), &job, metav1.CreateOptions{})

	if err != nil {
		log.Errorf("%v", err)
	}
	return jobApplied, err
}

func cleanup(client kubernetes.Interface, namespace string, jobApplied *batchv1.Job) error {
	err := client.BatchV1().Jobs(namespace).Delete(context.TODO(), jobApplied.Name, metav1.DeleteOptions{})
	return err
}
