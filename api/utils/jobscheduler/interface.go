package jobscheduler

import (
	batchesv1 "github.com/equinor/radix-job-scheduler/api/v1/batches"
	jobsv1 "github.com/equinor/radix-job-scheduler/api/v1/jobs"
	"github.com/equinor/radix-job-scheduler/models"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// HandlerFactoryInterface defines methods for creating job scheduler batch handler
type HandlerFactoryInterface interface {
	// CreateJobSchedulerBatchHandlerForEnv Created Job Scheduler batch handler for an environment
	CreateJobSchedulerBatchHandlerForEnv(env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) batchesv1.BatchHandler
	// CreateJobSchedulerJobHandlerForEnv Created Job Scheduler job handler for an environment
	CreateJobSchedulerJobHandlerForEnv(env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) jobsv1.JobHandler
}
