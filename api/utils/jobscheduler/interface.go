package jobscheduler

import (
	batchesv1 "github.com/equinor/radix-job-scheduler/api/v1/batches"
	"github.com/equinor/radix-job-scheduler/models"
)

// Interface defines methods to validate certificate and private key for TLS
type Interface interface {
	// CreateJobSchedulerBatchHandlerForEnv Created Job Scheduler batch handler for an environment
	CreateJobSchedulerBatchHandlerForEnv(env *models.Env) batchesv1.BatchHandler
}
