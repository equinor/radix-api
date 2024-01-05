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
	pipelineTagEnvironmentVariable    = "PIPELINE_IMG_TAG"
	radixTektonTagEnvironmentVariable = "RADIX_TEKTON_TAG"
)

// HandleStartPipelineJob Handles the creation of a pipeline job for an application
func (jh JobHandler) HandleStartPipelineJob(ctx context.Context, appName string, pipeline *pipelineJob.Definition, jobParameters *jobModels.JobParameters) (*jobModels.JobSummary, error) {
	radixRegistration, _ := jh.userAccount.RadixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})

	radixConfigFullName, err := getRadixConfigFullName(radixRegistration)
	if err != nil {
		return nil, err
	}

	job := jh.buildPipelineJob(appName, radixRegistration.Spec.CloneURL, radixConfigFullName, pipeline, jobParameters)
	return jh.createPipelineJob(ctx, appName, job)
}

func (jh JobHandler) createPipelineJob(ctx context.Context, appName string, job *v1.RadixJob) (*jobModels.JobSummary, error) {
	log.Infof("Starting job: %s, %s", job.GetName(), workerImage)
	appNamespace := k8sObjectUtils.GetAppNamespace(appName)
	job, err := jh.userAccount.RadixClient.RadixV1().RadixJobs(appNamespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	metrics.AddJobTriggered(appName, string(job.Spec.PipeLineType))

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

func (jh JobHandler) buildPipelineJob(appName, cloneURL, radixConfigFullName string, pipeline *pipelineJob.Definition, jobSpec *jobModels.JobParameters) *v1.RadixJob {
	jobName, imageTag := getUniqueJobName(workerImage)
	if len(jobSpec.ImageTag) > 0 {
		imageTag = jobSpec.ImageTag
	}

	var buildSpec v1.RadixBuildSpec
	var promoteSpec v1.RadixPromoteSpec
	var deploySpec v1.RadixDeploySpec

	triggeredBy, err := jh.getTriggeredBy(jobSpec.TriggeredBy)
	if err != nil {
		log.Warnf("failed to get triggeredBy: %v", err)
	}

	switch pipeline.Type {
	case v1.BuildDeploy, v1.Build:
		buildSpec = v1.RadixBuildSpec{
			ImageTag:  imageTag,
			Branch:    jobSpec.Branch,
			CommitID:  jobSpec.CommitID,
			PushImage: jobSpec.PushImage,
		}
	case v1.Promote:
		promoteSpec = v1.RadixPromoteSpec{
			DeploymentName:  jobSpec.DeploymentName,
			FromEnvironment: jobSpec.FromEnvironment,
			ToEnvironment:   jobSpec.ToEnvironment,
			CommitID:        jobSpec.CommitID,
		}
	case v1.Deploy:
		deploySpec = v1.RadixDeploySpec{
			ToEnvironment: jobSpec.ToEnvironment,
			ImageTagNames: jobSpec.ImageTagNames,
			CommitID:      jobSpec.CommitID,
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
			TektonImage:         getRadixTektonTag(),
			Build:               buildSpec,
			Promote:             promoteSpec,
			Deploy:              deploySpec,
			TriggeredBy:         triggeredBy,
			RadixConfigFullName: fmt.Sprintf("/workspace/%s", radixConfigFullName),
		},
	}

	return &job
}

func (jh JobHandler) getTriggeredBy(triggeredBy string) (string, error) {
	if triggeredBy != "" && triggeredBy != "<nil>" {
		return triggeredBy, nil
	}
	triggeredBy, err := jh.accounts.GetOriginator()
	if err != nil {
		return "", fmt.Errorf("failed to get originator: %w", err)
	}
	return triggeredBy, nil
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

func getRadixTektonTag() string {
	tektonTag := os.Getenv(radixTektonTagEnvironmentVariable)
	if tektonTag == "" {
		log.Warning("No radix tektom image tag defined. Using release-latest")
		tektonTag = "release-latest"
	} else {
		log.Infof("Using %s radix tekton image tag", tektonTag)
	}
	return tektonTag
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
