package jobs

import (
	"os"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"fmt"
	"math/rand"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

const workerImage = "radix-pipeline"
const dockerRegistry = "radixdev.azurecr.io"

// HandleGetApplicationJobDetails Handler for GetApplicationJobDetails
func HandleGetApplicationJobDetails(client kubernetes.Interface, appName string) ([]PipelineJob, error) {
	jobList, err := client.BatchV1().Jobs(getAppNamespace(appName)).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	jobs := make([]PipelineJob, len(jobList.Items))
	for i, job := range jobList.Items {
		jobs[i] = PipelineJob{Name: job.Name}
	}

	return jobs, nil
}

// HandleStartPipelineJob Handles the creation of a pipeline job for an application
func HandleStartPipelineJob(client kubernetes.Interface, appName string, jobSpec *PipelineJob) error {
	jobName, randomNr := getUniqueJobName(workerImage)
	job := createPipelineJob(jobName, randomNr, jobSpec.SSHRepo, jobSpec.Branch)

	log.Infof("Starting pipeline: %s, %s", jobName, workerImage)
	appNamespace := fmt.Sprintf("%s-app", appName)
	job, err := client.BatchV1().Jobs(appNamespace).Create(job)
	if err != nil {
		return err
	}

	log.Infof("Started pipeline: %s, %s", jobName, workerImage)
	jobSpec.Name = jobName
	return nil
}

// TODO : Separate out into library functions
func getAppNamespace(appName string) string {
	return fmt.Sprintf("%s-app", appName)
}

func createPipelineJob(jobName, randomStr, sshURL, pushBranch string) *batchv1.Job {
	pipelineTag := os.Getenv("PIPELINE_IMG_TAG")
	if pipelineTag == "" {
		pipelineTag = "latest"
	}

	imageTag := fmt.Sprintf("%s/%s:%s", dockerRegistry, workerImage, pipelineTag)
	log.Infof("Using image: %s", imageTag)
	cloneContainer, volume := CloneContainer(sshURL, "master")

	backOffLimit := int32(4)

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				"job_label": jobName,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "radix-pipeline",
					InitContainers: []corev1.Container{
						cloneContainer,
					},
					Containers: []corev1.Container{
						{
							Name:  workerImage,
							Image: imageTag,
							Args: []string{
								fmt.Sprintf("BRANCH=%s", pushBranch),
								fmt.Sprintf("RADIX_FILE_NAME=%s", "/workspace/radixconfig.yaml"),
								fmt.Sprintf("IMAGE_TAG=%s", randomStr),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "build-context",
									MountPath: "/workspace",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "build-context",
						},
						volume,
					},
					// https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/
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

	return &job
}

func CloneContainer(sshURL, branch string) (corev1.Container, corev1.Volume) {
	gitCloneCommand := fmt.Sprintf("git clone %s -b %s .", sshURL, branch)
	gitSSHKeyName := "git-ssh-keys"
	defaultMode := int32(256)
	container := corev1.Container{
		Name:    "clone",
		Image:   "radixdev.azurecr.io/gitclone:latest",
		Command: []string{"/bin/sh", "-c"},
		Args:    []string{gitCloneCommand},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "build-context",
				MountPath: "/workspace",
			},
			{
				Name:      gitSSHKeyName,
				MountPath: "/root/.ssh",
				ReadOnly:  true,
			},
		},
	}
	volume := corev1.Volume{
		Name: gitSSHKeyName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  gitSSHKeyName,
				DefaultMode: &defaultMode,
			},
		},
	}
	return container, volume
}

func getUniqueJobName(image string) (string, string) {
	var jobName []string
	randomStr := strings.ToLower(randStringBytesMaskImprSrc(5))
	jobName = append(jobName, image)
	jobName = append(jobName, "-")
	jobName = append(jobName, getCurrentTimestamp())
	jobName = append(jobName, "-")
	jobName = append(jobName, randomStr)
	return strings.Join(jobName, ""), randomStr
}

func getCurrentTimestamp() string {
	t := time.Now()
	return t.Format("20060102150405") // YYYYMMDDHHMISS in Go
}

var src = rand.NewSource(time.Now().UnixNano())

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func randStringBytesMaskImprSrc(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}
