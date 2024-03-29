package jobscheduler

import (
	batchesv1 "github.com/equinor/radix-job-scheduler/api/v1/batches"
	jobsv1 "github.com/equinor/radix-job-scheduler/api/v1/jobs"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
)

type factory struct {
	kubeUtil *kube.Kube
}

func (f *factory) CreateJobSchedulerBatchHandlerForEnv(env *models.Env) batchesv1.BatchHandler {
	return batchesv1.New(f.kubeUtil, env)
}

func (f *factory) CreateJobSchedulerJobHandlerForEnv(env *models.Env) jobsv1.JobHandler {
	return jobsv1.New(f.kubeUtil, env)
}

// NewFactory Constructor for factory
func NewFactory(kubeUtil *kube.Kube) HandlerFactoryInterface {
	return &factory{kubeUtil}
}
