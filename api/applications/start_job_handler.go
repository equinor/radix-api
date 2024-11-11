package applications

import (
	"context"
	"fmt"

	jobController "github.com/equinor/radix-api/api/jobs"
	jobModels "github.com/equinor/radix-api/api/jobs/models"
	"github.com/equinor/radix-api/api/middleware/auth"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	pipelineJob "github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	k8sObjectUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HandleStartPipelineJob Handles the creation of a pipeline jobController for an application
func HandleStartPipelineJob(ctx context.Context, radixClient versioned.Interface, appName, pipelineImageTag, tektonImageTag string, pipeline *pipelineJob.Definition, jobParameters *jobModels.JobParameters) (*jobModels.JobSummary, error) {
	radixRegistration, _ := radixClient.RadixV1().RadixRegistrations().Get(ctx, appName, metav1.GetOptions{})

	radixConfigFullName, err := getRadixConfigFullName(radixRegistration)
	if err != nil {
		return nil, err
	}

	job := buildPipelineJob(ctx, appName, radixRegistration.Spec.CloneURL, radixConfigFullName, pipelineImageTag, tektonImageTag, pipeline, jobParameters)
	return createPipelineJob(ctx, radixClient, appName, job)
}

func createPipelineJob(ctx context.Context, radixClient versioned.Interface, appName string, job *v1.RadixJob) (*jobModels.JobSummary, error) {
	log.Ctx(ctx).Info().Msgf("Starting jobController: %s, %s", job.GetName(), jobController.WorkerImage)
	appNamespace := k8sObjectUtils.GetAppNamespace(appName)
	job, err := radixClient.RadixV1().RadixJobs(appNamespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	log.Ctx(ctx).Info().Msgf("Started jobController: %s, %s", job.GetName(), jobController.WorkerImage)
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

func buildPipelineJob(ctx context.Context, appName, cloneURL, radixConfigFullName, pipelineImageTag, tektonImageTag string, pipeline *pipelineJob.Definition, jobSpec *jobModels.JobParameters) *v1.RadixJob {
	jobName, imageTag := jobController.GetUniqueJobName()
	if len(jobSpec.ImageTag) > 0 {
		imageTag = jobSpec.ImageTag
	}

	var buildSpec v1.RadixBuildSpec
	var promoteSpec v1.RadixPromoteSpec
	var deploySpec v1.RadixDeploySpec

	log.Ctx(ctx).Info().Msgf("Using %s pipeline image tag", pipelineImageTag)
	log.Ctx(ctx).Info().Msgf("Using %s as tekton image tag", tektonImageTag)

	switch pipeline.Type {
	case v1.BuildDeploy, v1.Build:
		buildSpec = v1.RadixBuildSpec{
			ImageTag:              imageTag,
			Branch:                jobSpec.Branch,
			ToEnvironment:         jobSpec.ToEnvironment,
			CommitID:              jobSpec.CommitID,
			PushImage:             jobSpec.PushImage,
			OverrideUseBuildCache: jobSpec.OverrideUseBuildCache,
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
			ToEnvironment:      jobSpec.ToEnvironment,
			ImageTagNames:      jobSpec.ImageTagNames,
			CommitID:           jobSpec.CommitID,
			ComponentsToDeploy: jobSpec.ComponentsToDeploy,
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
			PipelineImage:       pipelineImageTag,
			TektonImage:         tektonImageTag,
			Build:               buildSpec,
			Promote:             promoteSpec,
			Deploy:              deploySpec,
			TriggeredBy:         getTriggeredBy(ctx, jobSpec.TriggeredBy),
			RadixConfigFullName: fmt.Sprintf("/workspace/%s", radixConfigFullName),
		},
	}

	return &job
}

func getTriggeredBy(ctx context.Context, triggeredBy string) string {
	if triggeredBy != "" && triggeredBy != "<nil>" {
		return triggeredBy
	}

	return auth.GetOriginator(ctx)
}
