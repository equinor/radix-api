package jobs

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"

	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/metrics"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	pipelineJob "github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
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
	radixRegistration, _ := jh.userAccount.RadixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, metav1.GetOptions{})

	radixConfigFullName, err := getRadixConfigFullName(radixRegistration)
	if err != nil {
		return nil, err
	}

	job := jh.createPipelineJob(appName, radixRegistration.Spec.CloneURL, radixConfigFullName, pipeline, jobSpec)

	log.Infof("Starting job: %s, %s", job.GetName(), workerImage)
	appNamespace := k8sObjectUtils.GetAppNamespace(appName)
	job, err = jh.serviceAccount.RadixClient.RadixV1().RadixJobs(appNamespace).Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	metrics.AddJobTriggered(appName, string(pipeline.Type))

	log.Infof("Started job: %s, %s", job.GetName(), workerImage)
	return jobModels.GetSummaryFromRadixJob(job), nil
}

func getRadixConfigFullName(radixRegistration *v1.RadixRegistration) (string, error) {
	if len(radixRegistration.Spec.RadixConfigFullName) == 0 {
		return defaults.DefaultRadixConfigFileName, nil
	}
	if err := radixvalidators.ValidateRadixConfigFullName(radixRegistration.Spec.RadixConfigFullName); err != nil {
		return "", err
	}
	return radixRegistration.Spec.RadixConfigFullName, nil
}

func (jh JobHandler) createPipelineJob(appName, cloneURL, radixConfigFullName string, pipeline *pipelineJob.Definition, jobSpec *jobModels.JobParameters) *v1.RadixJob {
	jobName, imageTag := getUniqueJobName(workerImage)
	if len(jobSpec.ImageTag) > 0 {
		imageTag = jobSpec.ImageTag
	}

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
			ImageTag:  imageTag,
			Branch:    jobSpec.Branch,
			CommitID:  jobSpec.CommitID,
			PushImage: jobSpec.PushImage,
			//TODO - add in updated radix-operator
			//ImageRepository: jobSpec.ImageRepository,
			//ImageName:       jobSpec.ImageName,
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
			ImageTags:     jobSpec.ImageTags,
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
			AppName:             appName,
			CloneURL:            cloneURL,
			PipeLineType:        pipeline.Type,
			PipelineImage:       getPipelineTag(),
			DockerRegistry:      dockerRegistry,
			Build:               buildSpec,
			Promote:             promoteSpec,
			Deploy:              deploySpec,
			TriggeredBy:         triggeredBy,
			RadixConfigFullName: fmt.Sprintf("/workspace/%s", radixConfigFullName),
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
	randomStr := strings.ToLower(radixutils.RandString(5))
	jobName = append(jobName, image, "-", getCurrentTimestamp(), "-", randomStr)

	return strings.Join(jobName, ""), randomStr
}

func getCurrentTimestamp() string {
	t := time.Now()
	return t.Format("20060102150405") // YYYYMMDDHHMISS in Go
}
