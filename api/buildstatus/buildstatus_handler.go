package buildstatus

import (
	"fmt"
	"sort"

	build_models "github.com/equinor/radix-api/api/buildstatus/models"
	"github.com/equinor/radix-api/api/utils"
	"github.com/equinor/radix-api/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BuildStatusHandler struct {
	accounts    models.Accounts
	buildstatus build_models.Status
}

func Init(accounts models.Accounts, status build_models.Status) BuildStatusHandler {
	return BuildStatusHandler{accounts: accounts, buildstatus: status}
}

// GetBuildStatusForApplication Gets a list of build status for environments
func (handler BuildStatusHandler) GetBuildStatusForApplication(appName, env string) (*[]byte, error) {
	var output []byte

	// Get latest RJ
	serviceAccount := handler.accounts.ServiceAccount
	namespace := fmt.Sprintf("%s-app", appName)

	// Get list of Jobs in the namespace
	radixJobs, err := serviceAccount.RadixClient.RadixV1().RadixJobs(namespace).List(metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	latestBuildDeployJob, err := getLatestBuildJobToEnvironment(radixJobs.Items, env)

	if err != nil {
		return nil, utils.NotFoundError(err.Error())
	}

	buildCondition := latestBuildDeployJob.Status.Condition

	if buildCondition == "Succeeded" {
		output = append(output, *handler.buildstatus.WriteSvg(build_models.BUILD_STATUS_PASSING)...)
	} else if buildCondition == "Failed" {
		output = append(output, *handler.buildstatus.WriteSvg(build_models.BUILD_STATUS_FAILING)...)
	} else if buildCondition == "Stopped" {
		output = append(output, *handler.buildstatus.WriteSvg(build_models.BUILD_STATUS_STOPPED)...)
	} else if buildCondition == "Waiting" || buildCondition == "Running" {
		output = append(output, *handler.buildstatus.WriteSvg(build_models.BUILD_STATUS_PENDING)...)
	} else {
		output = append(output, *handler.buildstatus.WriteSvg(build_models.BUILD_STATUS_UNKNOWN)...)
	}

	return &output, nil
}

func getLatestBuildJobToEnvironment(jobs []v1.RadixJob, env string) (v1.RadixJob, error) {
	// Filter out all BuildDeploy jobs
	allBuildDeployJobs := []v1.RadixJob{}
	for _, job := range jobs {
		if job.Spec.PipeLineType == v1.BuildDeploy {
			allBuildDeployJobs = append(allBuildDeployJobs, job)
		}
	}

	// Sort the slice by created date (In descending order)
	sort.Slice(allBuildDeployJobs[:], func(i, j int) bool {
		return allBuildDeployJobs[j].Status.Created.Before(allBuildDeployJobs[i].Status.Created)
	})

	// Get status of the last job to requested environment
	for _, buildDeployJob := range allBuildDeployJobs {
		for _, targetEnvironment := range buildDeployJob.Status.TargetEnvs {
			if targetEnvironment == env {
				return buildDeployJob, nil
			}
		}
	}

	return v1.RadixJob{}, fmt.Errorf("No build-deploy jobs were found in %s environment", env)

}
