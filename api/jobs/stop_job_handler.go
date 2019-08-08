package jobs

import (
	"encoding/json"
	"fmt"

	jobModels "github.com/equinor/radix-api/api/jobs/models"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// StopJob Stops an application job
func (jh JobHandler) StopJob(appName, jobName string) error {
	log.Infof("Stopping job: %s, %s", jobName, appName)
	appNamespace := crdUtils.GetAppNamespace(appName)
	job, err := jh.serviceAccount.RadixClient.RadixV1().RadixJobs(appNamespace).Get(jobName, metav1.GetOptions{})

	if errors.IsNotFound(err) {
		return jobModels.PipelineNotFoundError(appName, jobName)
	}
	if err != nil {
		return err
	}

	newJob := job.DeepCopy()
	newJob.Spec.Stop = true

	jobJSON, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("Failed to marshal old job object: %v", err)
	}

	newJobJSON, err := json.Marshal(newJob)
	if err != nil {
		return fmt.Errorf("Failed to marshal new job object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(jobJSON, newJobJSON, v1.RadixJob{})
	if err != nil {
		return fmt.Errorf("Failed to create two way merge patch job objects: %v", err)
	}

	_, err = jh.userAccount.RadixClient.RadixV1().RadixJobs(appNamespace).Patch(jobName, types.StrategicMergePatchType, patchBytes)
	if err != nil {
		return fmt.Errorf("Failed to patch job object: %v", err)
	}

	return nil
}
