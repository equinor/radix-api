package applications

import (
	"fmt"
	"strings"

	"github.com/statoil/radix-operator/pkg/apis/kube"

	log "github.com/Sirupsen/logrus"
	radixjob "github.com/statoil/radix-api/api/jobs"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
)

func IsDeployKeyValid(client kubernetes.Interface, radixclient radixclient.Interface, appName string) (bool, error) {
	rr, err := radixclient.RadixV1().RadixRegistrations("default").Get(appName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	namespace := kube.GetCiCdNamespace(rr)
	jobApplied, err := applyCloneJob(client, rr)

	w, err := client.BatchV1().Jobs(jobApplied.Namespace).Watch(metav1.ListOptions{
		FieldSelector: fields.Set{"metadata.name": jobApplied.Name}.AsSelector().String(),
	})
	if err != nil {
		return false, err
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
					_ = client.BatchV1().Jobs(namespace).Delete(jobApplied.Name, &metav1.DeleteOptions{})
					done <- nil
					return
				case j.Status.Failed == 1:
					done <- fmt.Errorf("Git clone failed")
					return
				default:
					log.Debugf("Ongoing - build docker image")
				}
			}
		}
	}()
	err = <-done

	return err == nil, err
}

func applyCloneJob(client kubernetes.Interface, rr *v1.RadixRegistration) (*batchv1.Job, error) {
	jobName := strings.ToLower(fmt.Sprintf("%s-%s", rr.Name, utils.RandString(5)))
	namespace := kube.GetCiCdNamespace(rr)
	backOffLimit := int32(1)

	cloneContainer, volume := radixjob.CloneContainer(rr.Spec.CloneURL, "master")

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				"job_label": jobName,
				"type":      "validate-clone-url",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "radix-pipeline",
					Containers: []corev1.Container{
						cloneContainer,
					},
					Volumes: []corev1.Volume{
						{
							Name: "build-context",
						},
						volume,
					},
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: "regcred",
						},
					},
					RestartPolicy: "Never",
				},
			},
		},
	}

	jobApplied, err := client.BatchV1().Jobs(namespace).Create(&job)

	if err != nil {
		log.Errorf("%v", err)
	}
	return jobApplied, err
}
