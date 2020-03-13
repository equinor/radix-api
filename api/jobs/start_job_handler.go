package jobs

import (
	"os"
	"strings"
	"time"

	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/metrics"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	pipelineJob "github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	pipelineTagEnvironmentVariable       = "PIPELINE_IMG_TAG"
	containerRegistryEnvironmentVariable = "RADIX_CONTAINER_REGISTRY"
)

// HandleStartPipelineJob Handles the creation of a pipeline job for an application
func (jh JobHandler) HandleStartPipelineJob(appName string, pipeline *pipelineJob.Definition, jobSpec *jobModels.JobParameters) (*jobModels.JobSummary, error) {
	radixRegistation, _ := jh.userAccount.RadixClient.RadixV1().RadixRegistrations().Get(appName, metav1.GetOptions{})

	job := jh.createPipelineJob(appName, radixRegistation.Spec.CloneURL, pipeline, jobSpec)

	log.Infof("Starting job: %s, %s", job.GetName(), workerImage)
	appNamespace := k8sObjectUtils.GetAppNamespace(appName)
	job, err := jh.serviceAccount.RadixClient.RadixV1().RadixJobs(appNamespace).Create(job)
	if err != nil {
		return nil, err
	}

	metrics.AddJobTriggered(appName, string(pipeline.Type))

	log.Infof("Started job: %s, %s", job.GetName(), workerImage)
	return jobModels.GetSummaryFromRadixJob(job), nil
}

func (jh JobHandler) createPipelineJob(appName, cloneURL string, pipeline *pipelineJob.Definition, jobSpec *jobModels.JobParameters) *v1.RadixJob {
	jobName, randomStr := getUniqueJobName(workerImage)
	dockerRegistry := os.Getenv(containerRegistryEnvironmentVariable)

	var buildSpec v1.RadixBuildSpec
	var promoteSpec v1.RadixPromoteSpec
	var deploySpec v1.RadixDeploySpec

	triggeredBy := jobSpec.TriggeredBy
	if triggeredBy == "" {
		triggeredBy, _ = jh.accounts.GetUserAccountUserPrincipleName()
	}
	if triggeredBy == "<nil>" {
		triggeredBy = ""
	}

	switch pipeline.Type {
	case v1.BuildDeploy, v1.Build:
		buildSpec = v1.RadixBuildSpec{
			ImageTag:      randomStr,
			Branch:        jobSpec.Branch,
			CommitID:      jobSpec.CommitID,
			PushImage:     jobSpec.PushImage,
			RadixFileName: "/workspace/radixconfig.yaml",
		}
	case v1.Promote:
		promoteSpec = v1.RadixPromoteSpec{
			DeploymentName:  jobSpec.DeploymentName,
			FromEnvironment: jobSpec.FromEnvironment,
			ToEnvironment:   jobSpec.ToEnvironment,
		}
	case v1.Deploy:
		deploySpec = v1.RadixDeploySpec{
			ToEnvironment: jobSpec.ToEnvironment,
		}
	}

	job := v1.RadixJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
			Annotations: map[string]string{
				kube.RadixBranchAnnotation: jobSpec.Branch,
			},
		},
		Spec: v1.RadixJobSpec{
			AppName:        appName,
			CloneURL:       cloneURL,
			PipeLineType:   pipeline.Type,
			PipelineImage:  getPipelineTag(),
			DockerRegistry: dockerRegistry,
			Build:          buildSpec,
			Promote:        promoteSpec,
			Deploy:         deploySpec,
			TriggeredBy:    triggeredBy,
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
