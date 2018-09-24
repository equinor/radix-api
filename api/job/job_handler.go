package job

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
)

const workerImage = "radix-pipeline"
const dockerRegistry = "radixdev.azurecr.io"

// HandleGetPipelineJobs Handler for GetPipelineJobs
func HandleGetPipelineJobs(client kubernetes.Interface) (*PipelineJobsResponse, error) {
	jobList, err := client.BatchV1().Jobs(corev1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	jobs := make([]PipelineJob, len(jobList.Items))
	for i, job := range jobList.Items {
		jobs[i] = PipelineJob{Name: job.Name}
	}

	return &PipelineJobsResponse{Jobs: jobs}, nil
}

// HandleCreatePipelineJob Handles the creation of a pipeline job for an application
func HandleCreatePipelineJob(client kubernetes.Interface, jobSpec *PipelineJob) error {
	jobName, randomNr := getUniqueJobName(workerImage)
	job := createPipelineJob(jobName, randomNr, jobSpec.SSHRepo, jobSpec.Branch)

	logrus.Infof("Starting pipeline: %s, %s", jobName, workerImage)
	appNamespace := fmt.Sprintf("%s-app", jobSpec.AppName)
	job, err := client.BatchV1().Jobs(appNamespace).Create(job)
	if err != nil {
		return err
	}

	logrus.Infof("Started pipeline: %s, %s", jobName, workerImage)
	return nil
}

func createPipelineJob(jobName, randomStr, sshURL, pushBranch string) *batchv1.Job {
	gitCloneCommand := fmt.Sprintf("git clone %s -b %s .", sshURL, "master")
	imageTag := fmt.Sprintf("%s/%s:%s", dockerRegistry, workerImage, "latest")
	logrus.Infof("Using image: %s", imageTag)

	backOffLimit := int32(4)
	defaultMode := int32(256)

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
						{
							Name:    "clone",
							Image:   "alpine:3.7",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{fmt.Sprintf("apk add --no-cache bash openssh-client git && ls /root/.ssh && cd /workspace && %s", gitCloneCommand)},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "build-context",
									MountPath: "/workspace",
								},
								{
									Name:      "git-ssh-keys",
									MountPath: "/root/.ssh",
									ReadOnly:  true,
								},
							},
						},
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
						{
							Name: "git-ssh-keys",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  "git-ssh-keys",
									DefaultMode: &defaultMode,
								},
							},
						},
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
