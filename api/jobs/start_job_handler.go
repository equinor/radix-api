package jobs

import (
	"fmt"
	"os"
	"strings"
	"time"

	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/metrics"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	pipelineTagEnvironmentVariable       = "PIPELINE_IMG_TAG"
	containerRegistryEnvironmentVariable = "RADIX_CONTAINER_REGISTRY"
)

// HandleStartPipelineJob Handles the creation of a pipeline job for an application
func (jh JobHandler) HandleStartPipelineJob(appName, sshRepo string, pipeline jobModels.Pipeline, jobSpec *jobModels.JobParameters) (*jobModels.JobSummary, error) {
	job := createPipelineJob(appName, sshRepo, pipeline, jobSpec)

	log.Infof("Starting job: %s, %s", job.GetName(), workerImage)
	appNamespace := fmt.Sprintf("%s-app", appName)
	job, err := jh.serviceAccount.Client.BatchV1().Jobs(appNamespace).Create(job)
	if err != nil {
		return nil, err
	}

	metrics.AddJobTriggered(appName, pipeline.String())

	log.Infof("Started job: %s, %s", job.GetName(), workerImage)
	return jobModels.GetJobSummary(job), nil
}

func createPipelineJob(appName, sshURL string, pipeline jobModels.Pipeline, jobSpec *jobModels.JobParameters) *batchv1.Job {
	backOffLimit := int32(0)
	pushBranch := jobSpec.Branch
	commitID := jobSpec.CommitID
	jobName, randomStr := getUniqueJobName(workerImage)
	dockerRegistry := os.Getenv(containerRegistryEnvironmentVariable)
	imageTag := fmt.Sprintf("%s/%s:%s", dockerRegistry, workerImage, getPipelineTag())
	pipelineType := pipeline.String()

	log.Infof("Using image: %s", imageTag)

	initContainers := git.CloneInitContainers(sshURL, "master")
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				kube.RadixJobNameLabel:  jobName,
				kube.RadixJobTypeLabel:  RadixJobTypeJob,
				"radix-pipeline":        pipelineType,
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
					InitContainers:     initContainers,
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
								fmt.Sprintf("PIPELINE_TYPE=%s", pipelineType),
								fmt.Sprintf("PUSH_IMAGE=%s", jobSpec.GetPushImageTag()),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      git.BuildContextVolumeName,
									MountPath: git.Workspace,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: git.Workspace,
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

func getPipelineTag() string {
	pipelineTag := os.Getenv(pipelineTagEnvironmentVariable)
	if pipelineTag == "" {
		log.Warning("No pipeline image tag defined. Using latest")
		pipelineTag = "latest"
	} else {
		log.Infof("Using %s pipeline image tag", pipelineTag)
	}
	return pipelineTag
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
