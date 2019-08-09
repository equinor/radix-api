package jobs

import (
	"fmt"
	"os"
	"strings"
	"time"

	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/metrics"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	pipelineJob "github.com/equinor/radix-operator/pkg/apis/pipeline"
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
func (jh JobHandler) HandleStartPipelineJob(appName, sshRepo string, pipeline *pipelineJob.Definition, jobSpec *jobModels.JobParameters) (*jobModels.JobSummary, error) {
	job := createPipelineJob(appName, sshRepo, pipeline, jobSpec)

	log.Infof("Starting job: %s, %s", job.GetName(), workerImage)
	appNamespace := fmt.Sprintf("%s-app", appName)
	job, err := jh.serviceAccount.Client.BatchV1().Jobs(appNamespace).Create(job)
	if err != nil {
		return nil, err
	}

	metrics.AddJobTriggered(appName, pipeline.Name)

	log.Infof("Started job: %s, %s", job.GetName(), workerImage)
	return jobModels.GetJobSummary(job), nil
}

func createPipelineJob(appName, sshURL string, pipeline *pipelineJob.Definition, jobSpec *jobModels.JobParameters) *batchv1.Job {
	backOffLimit := int32(0)
	jobName, randomStr := getUniqueJobName(workerImage)
	dockerRegistry := os.Getenv(containerRegistryEnvironmentVariable)
	imageTag := fmt.Sprintf("%s/%s:%s", dockerRegistry, workerImage, getPipelineTag())

	log.Infof("Using image: %s", imageTag)

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: getPipelineJobLabels(appName, jobName, randomStr, pipeline, jobSpec),
			Annotations: map[string]string{
				kube.RadixBranchAnnotation: jobSpec.Branch,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "radix-pipeline",
					InitContainers:     getPipelineJobInitContainers(sshURL, pipeline),
					Containers: []corev1.Container{
						{
							Name:            workerImage,
							Image:           imageTag,
							ImagePullPolicy: corev1.PullAlways,
							Args:            getPipelineJobArguments(appName, jobName, randomStr, pipeline, jobSpec),
							VolumeMounts:    getPipelineJobContainerVolumeMounts(pipeline),
						},
					},
					Volumes:       getPipelineJobVolumes(pipeline),
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

func getPipelineJobInitContainers(sshURL string, pipeline *pipelineJob.Definition) []corev1.Container {
	var initContainers []corev1.Container

	switch pipeline.Name {
	case pipelineJob.BuildDeploy, pipelineJob.Build:
		initContainers = git.CloneInitContainers(sshURL, "master")
	}
	return initContainers
}

func getPipelineJobArguments(appName string, jobName string, randomStr string, pipeline *pipelineJob.Definition, jobSpec *jobModels.JobParameters) []string {
	// Base arguments for all types of pipeline
	args := []string{
		fmt.Sprintf("JOB_NAME=%s", jobName),
		fmt.Sprintf("PIPELINE_TYPE=%s", pipeline.Name),
	}

	switch pipeline.Name {
	case pipelineJob.BuildDeploy:
		args = append(args, fmt.Sprintf("IMAGE_TAG=%s", randomStr))
		fallthrough
	case pipelineJob.Build:
		args = append(args, fmt.Sprintf("BRANCH=%s", jobSpec.Branch))
		args = append(args, fmt.Sprintf("COMMIT_ID=%s", jobSpec.CommitID))
		args = append(args, fmt.Sprintf("PUSH_IMAGE=%s", jobSpec.GetPushImageTag()))
		args = append(args, fmt.Sprintf("RADIX_FILE_NAME=%s", "/workspace/radixconfig.yaml"))
	case pipelineJob.Promote:
		args = append(args, fmt.Sprintf("RADIX_APP=%s", appName))
		args = append(args, fmt.Sprintf("DEPLOYMENT_NAME=%s", jobSpec.DeploymentName))
		args = append(args, fmt.Sprintf("FROM_ENVIRONMENT=%s", jobSpec.FromEnvironment))
		args = append(args, fmt.Sprintf("TO_ENVIRONMENT=%s", jobSpec.ToEnvironment))
	}

	return args
}

func getPipelineJobLabels(appName string, jobName string, randomStr string, pipeline *pipelineJob.Definition, jobSpec *jobModels.JobParameters) map[string]string {
	// Base labels for all types of pipeline
	labels := map[string]string{
		kube.RadixJobNameLabel: jobName,
		kube.RadixJobTypeLabel: RadixJobTypeJob,
		"radix-pipeline":       pipeline.Name,
		"radix-app-name":       appName, // For backwards compatibility. Remove when cluster is migrated
		kube.RadixAppLabel:     appName,
	}

	switch pipeline.Name {
	case pipelineJob.BuildDeploy:
		labels[kube.RadixImageTagLabel] = randomStr
		fallthrough
	case pipelineJob.Build:
		labels[kube.RadixCommitLabel] = jobSpec.CommitID
	}

	return labels
}

func getPipelineJobContainerVolumeMounts(pipeline *pipelineJob.Definition) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount

	switch pipeline.Name {
	case pipelineJob.BuildDeploy, pipelineJob.Build:
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      git.BuildContextVolumeName,
			MountPath: git.Workspace,
		})
	}

	return volumeMounts
}

func getPipelineJobVolumes(pipeline *pipelineJob.Definition) []corev1.Volume {
	var volumes []corev1.Volume
	defaultMode := int32(256)

	switch pipeline.Name {
	case pipelineJob.BuildDeploy, pipelineJob.Build:
		volumes = append(volumes, corev1.Volume{
			Name: git.BuildContextVolumeName,
		})
		volumes = append(volumes, corev1.Volume{
			Name: git.GitSSHKeyVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  git.GitSSHKeyVolumeName,
					DefaultMode: &defaultMode,
				},
			},
		})
	}

	return volumes
}
