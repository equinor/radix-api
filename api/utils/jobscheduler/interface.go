package jobscheduler

import (
	batchesv1 "github.com/equinor/radix-job-scheduler/api/v1/batches"
	"github.com/equinor/radix-job-scheduler/models"
)

// HandlerFactoryInterface defines methods for creating job scheduler batch handler
type HandlerFactoryInterface interface {
	// CreateJobSchedulerBatchHandlerForEnv Created Job Scheduler batch handler for an environment
	CreateJobSchedulerBatchHandlerForEnv(env *models.Env) batchesv1.BatchHandler
}
