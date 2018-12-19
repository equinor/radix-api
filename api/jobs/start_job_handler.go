package jobs

import (
	"fmt"
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	jobModels "github.com/statoil/radix-api/api/jobs/models"
	"github.com/statoil/radix-operator/pkg/apis/kube"
	"github.com/statoil/radix-operator/pkg/apis/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HandleStartPipelineJob Handles the creation of a pipeline job for an application
func (jh JobHandler) HandleStartPipelineJob(appName, sshRepo string, pipeline jobModels.Pipeline, jobSpec *jobModels.JobParameters) (*jobModels.JobSummary, error) {
	jobName, randomNr := getUniqueJobName(workerImage)
	job := createPipelineJob(appName, jobName, randomNr, sshRepo, pipeline, jobSpec.Branch, jobSpec.CommitID)

	log.Infof("Starting job: %s, %s", jobName, workerImage)
	appNamespace := fmt.Sprintf("%s-app", appName)
	job, err := jh.client.BatchV1().Jobs(appNamespace).Create(job)
	if err != nil {
		return nil, err
	}

	log.Infof("Started job: %s, %s", jobName, workerImage)
	return GetJobSummary(job), nil
}

// CloneContainer The sidecar for cloning repo
func CloneContainer(sshURL, branch string) (corev1.Container, corev1.Volume) {
	gitCloneCommand := fmt.Sprintf("git clone %s -b %s --verbose --progress .", sshURL, branch)
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

func createPipelineJob(appName, jobName, randomStr, sshURL string, pipeline jobModels.Pipeline, pushBranch, commitID string) *batchv1.Job {
	pipelineTag := os.Getenv("PIPELINE_IMG_TAG")
	if pipelineTag == "" {
		log.Warning("No pipeline image tag defined. Using latest")
		pipelineTag = "latest"
	} else {
		log.Infof("Using %s pipeline image tag", pipelineTag)
	}

	imageTag := fmt.Sprintf("%s/%s:%s", dockerRegistry, workerImage, pipelineTag)
	log.Infof("Using image: %s", imageTag)
	cloneContainer, volume := CloneContainer(sshURL, "master")

	backOffLimit := int32(1)

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				kube.RadixJobNameLabel:  jobName,
				kube.RadixJobTypeLabel:  RadixJobTypeJob,
				"radix-pipeline":        pipeline.String(),
				"radix-app-name":        appName, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel:      appName,
				kube.RadixBranchLabel:   pushBranch,
				kube.RadixImageTagLabel: randomStr,
				kube.RadixCommitLabel:   commitID,
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
							Name:            workerImage,
							Image:           imageTag,
							ImagePullPolicy: corev1.PullAlways,
							Args: []string{
								fmt.Sprintf("BRANCH=%s", pushBranch),
								fmt.Sprintf("COMMIT_ID=%s", commitID),
								fmt.Sprintf("RADIX_FILE_NAME=%s", "/workspace/radixconfig.yaml"),
								fmt.Sprintf("IMAGE_TAG=%s", randomStr),
								fmt.Sprintf("JOB_NAME=%s", jobName),
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

func getUniqueJobName(image string) (string, string) {
	var jobName []string
	randomStr := strings.ToLower(utils.RandString(5))
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
