package jobscheduler

import (
	batchesv1 "github.com/equinor/radix-job-scheduler/api/v1/batches"
	jobsv1 "github.com/equinor/radix-job-scheduler/api/v1/jobs"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type factory struct {
	kubeUtil *kube.Kube
}

func (f *factory) CreateJobSchedulerBatchHandlerForEnv(env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) batchesv1.BatchHandler {
	return batchesv1.New(f.kubeUtil, env, radixDeployJobComponent)
}

func (f *factory) CreateJobSchedulerJobHandlerForEnv(env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) jobsv1.JobHandler {
	return jobsv1.New(f.kubeUtil, env, radixDeployJobComponent)
}

// NewFactory Constructor for factory
func NewFactory(kubeUtil *kube.Kube) HandlerFactoryInterface {
	return &factory{kubeUtil}
}
