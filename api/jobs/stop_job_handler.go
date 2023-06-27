package jobs

import (
	"context"
	"fmt"

	jobModels "github.com/equinor/radix-api/api/jobs/models"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StopJob Stops an application job
func (jh JobHandler) StopJob(ctx context.Context, appName, jobName string) error {
	log.Infof("Stopping job: %s, %s", jobName, appName)
	appNamespace := crdUtils.GetAppNamespace(appName)
	job, err := jh.serviceAccount.RadixClient.RadixV1().RadixJobs(appNamespace).Get(ctx, jobName, metav1.GetOptions{})

	if errors.IsNotFound(err) {
		return jobModels.PipelineNotFoundError(appName, jobName)
	}
	if err != nil {
		return err
	}

	job.Spec.Stop = true

	_, err = jh.userAccount.RadixClient.RadixV1().RadixJobs(appNamespace).Update(ctx, job, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch job object: %v", err)
	}

	return nil
}
