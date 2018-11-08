package jobs

import (
	"bytes"
	"os"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"fmt"
	"math/rand"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	jobModels "github.com/statoil/radix-api/api/jobs/models"
	"github.com/statoil/radix-api/api/utils"
	crdUtils "github.com/statoil/radix-operator/pkg/apis/utils"
)

const workerImage = "radix-pipeline"
const dockerRegistry = "radixdev.azurecr.io"

// HandleGetApplicationJobLogs Gets logs for an job of an application
func HandleGetApplicationJobLogs(client kubernetes.Interface, appName, jobName string) (string, error) {
	ns := crdUtils.GetAppNamespace(appName)

	pods, err := client.CoreV1().Pods(ns).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})

	if err != nil {
		return "", err
	}

	pipelinePod := pods.Items[0]

	cloneLog, err := handleGetPodLog(client, &pipelinePod, "clone")
	if err != nil {
		return "", err
	}

	podLog, err := handleGetPodLog(client, &pipelinePod, "")
	if err != nil {
		return "", err
	}

	imageTag := strings.TrimPrefix(pipelinePod.Spec.Containers[0].Args[2], "IMAGE_TAG=")
	buildJobName := fmt.Sprintf("radix-builder-%s", imageTag)

	pods, err = client.CoreV1().Pods(ns).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", buildJobName),
	})
	if err != nil {
		return "", err
	}
	sbBuildersLog := strings.Builder{}
	for _, buildPod := range pods.Items {
		buildLog, err := handleGetPodLog(client, &buildPod, "")
		if err != nil {
			return "", err
		}
		sbBuildersLog.WriteString(buildLog)
	}

	return fmt.Sprintln(cloneLog, podLog, sbBuildersLog.String()), nil
}

// HandleGetApplicationJobs Handler for GetApplicationJobs
func HandleGetApplicationJobs(client kubernetes.Interface, appName string) ([]jobModels.JobSummary, error) {
	jobList, err := client.BatchV1().Jobs(crdUtils.GetAppNamespace(appName)).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("radix-job-type=%s", "job"),
	})
	if err != nil {
		return nil, err
	}

	jobs := make([]jobModels.JobSummary, len(jobList.Items))
	for i, job := range jobList.Items {
		jobs[i] = *GetJobSummary(&job)
	}

	return jobs, nil
}

// HandleGetApplicationJob Handler for GetApplicationJob
func HandleGetApplicationJob(client kubernetes.Interface, appName, jobName string) (*jobModels.Job, error) {
	job, err := client.BatchV1().Jobs(crdUtils.GetAppNamespace(appName)).Get(jobName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !strings.EqualFold(job.Labels["radix-job-type"], "job") {
		return nil, utils.ValidationError("Radix Application Job", "Job was not of expected type")
	}

	labelSelector := fmt.Sprintf("radix-image-tag=%s, radix-job-type!=%s", job.Labels["radix-image-tag"], "job")
	jobStepList, err := client.BatchV1().Jobs(crdUtils.GetAppNamespace(appName)).List(metav1.ListOptions{
		LabelSelector: labelSelector,
	})

	steps := make([]jobModels.Step, 0)
	for _, jobStep := range jobStepList.Items {
		jobStepPod, err := client.CoreV1().Pods(crdUtils.GetAppNamespace(appName)).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("job-name=%s", jobStep.Name),
		})

		if err != nil {
			return nil, err
		}

		if len(jobStepPod.Items) == 0 {
			continue
		}

		pod := jobStepPod.Items[0]
		for _, containerStatus := range pod.Status.InitContainerStatuses {
			steps = append(steps, getJobStep(&containerStatus))
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			steps = append(steps, getJobStep(&containerStatus))
		}

	}

	branch := job.Labels["radix-branch"]
	commit := job.Labels["radix-commit"]
	pipeline := job.Labels["radix-pipeline"]

	jobStatus := jobModels.GetStatusFromJobStatus(job.Status)
	var jobEnded metav1.Time

	if len(job.Status.Conditions) > 0 {
		jobEnded = job.Status.Conditions[0].LastTransitionTime
	}

	return &jobModels.Job{
		Name:     job.Name,
		Branch:   branch,
		CommitID: commit,
		Started:  utils.FormatTime(job.Status.StartTime),
		Ended:    utils.FormatTime(&jobEnded),
		Status:   jobStatus.String(),
		Pipeline: pipeline,
		Steps:    steps,
	}, nil
}

// HandleStartPipelineJob Handles the creation of a pipeline job for an application
func HandleStartPipelineJob(client kubernetes.Interface, appName, sshRepo string, pipeline jobModels.Pipeline, jobSpec *jobModels.JobParameters) (*jobModels.JobSummary, error) {
	jobName, randomNr := getUniqueJobName(workerImage)
	job := createPipelineJob(appName, jobName, randomNr, sshRepo, pipeline, jobSpec.Branch, jobSpec.CommitID)

	log.Infof("Starting job: %s, %s", jobName, workerImage)
	appNamespace := fmt.Sprintf("%s-app", appName)
	job, err := client.BatchV1().Jobs(appNamespace).Create(job)
	if err != nil {
		return nil, err
	}

	log.Infof("Started job: %s, %s", jobName, workerImage)
	return GetJobSummary(job), nil
}

// GetJobSummary Used to get job summary from a kubernetes job
func GetJobSummary(job *batchv1.Job) *jobModels.JobSummary {
	appName := job.Labels["radix-app-name"]
	branch := job.Labels["radix-branch"]
	commit := job.Labels["radix-commit"]
	status := job.Status

	jobStatus := jobModels.GetStatusFromJobStatus(status)
	ended := utils.FormatTime(status.CompletionTime)
	if jobStatus == jobModels.Failed {
		ended = utils.FormatTime(&status.Conditions[0].LastTransitionTime)
	}

	pipelineJob := &jobModels.JobSummary{
		Name:     job.Name,
		AppName:  appName,
		Branch:   branch,
		CommitID: commit,
		Status:   jobStatus.String(),
		Started:  utils.FormatTime(status.StartTime),
		Ended:    ended,
	}
	return pipelineJob
}

// CloneContainer The sidecar for cloning repo
func CloneContainer(sshURL, branch string) (corev1.Container, corev1.Volume) {
	gitCloneCommand := fmt.Sprintf("git clone %s -b %s -v .", sshURL, branch)
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

func getJobStep(containerStatus *corev1.ContainerStatus) jobModels.Step {
	var startedAt metav1.Time
	var finishedAt metav1.Time

	status := jobModels.Succeeded

	if containerStatus.State.Terminated != nil {
		startedAt = containerStatus.State.Terminated.StartedAt
		finishedAt = containerStatus.State.Terminated.FinishedAt

		if containerStatus.State.Terminated.ExitCode > 0 {
			status = jobModels.Failed
		}

	} else if containerStatus.State.Running != nil {
		startedAt = containerStatus.State.Running.StartedAt
		status = jobModels.Active

	} else if containerStatus.State.Waiting != nil {
		status = jobModels.Waiting

	}

	return jobModels.Step{
		Name:    containerStatus.Name,
		Started: utils.FormatTime(&startedAt),
		Ended:   utils.FormatTime(&finishedAt),
		Status:  status.String(),
	}
}

func handleGetPodLog(client kubernetes.Interface, pod *corev1.Pod, containerName string) (string, error) {
	req := getPodLogRequest(client, pod, containerName, false)

	readCloser, err := req.Stream()
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(readCloser)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func getPodLogRequest(client kubernetes.Interface, pod *corev1.Pod, containerName string, follow bool) *rest.Request {
	podLogOption := corev1.PodLogOptions{
		Follow: follow,
	}
	if containerName != "" {
		podLogOption.Container = containerName
	}

	req := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOption)
	return req
}

func createPipelineJob(appName, jobName, randomStr, sshURL string, pipeline jobModels.Pipeline, pushBranch, commitID string) *batchv1.Job {
	pipelineTag := os.Getenv("PIPELINE_IMG_TAG")
	if pipelineTag == "" {
		pipelineTag = "latest"
	}

	imageTag := fmt.Sprintf("%s/%s:%s", dockerRegistry, workerImage, pipelineTag)
	log.Infof("Using image: %s", imageTag)
	CloneContainer, volume := CloneContainer(sshURL, "master")

	backOffLimit := int32(1)

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				"radix-job-label": jobName,
				"radix-job-type":  "job",
				"radix-pipeline":  pipeline.String(),
				"radix-app-name":  appName,
				"radix-branch":    pushBranch,
				"radix-image-tag": randomStr,
				"radix-commit":    commitID,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "radix-pipeline",
					InitContainers: []corev1.Container{
						CloneContainer,
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
