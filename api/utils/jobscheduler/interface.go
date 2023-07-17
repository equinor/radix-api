package jobscheduler

import (
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	"github.com/equinor/radix-job-scheduler/models"
)

// Interface defines methods to validate certificate and private key for TLS
type Interface interface {
	// CreateJobSchedulerHandlerForEnv Created Job Scheduler handler for an environment
	CreateJobSchedulerHandlerForEnv(*models.Env) apiv2.Handler
}
