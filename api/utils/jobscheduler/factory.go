package jobscheduler

import (
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
)

type factory struct {
	kubeUtil *kube.Kube
}

func (f *factory) CreateJobSchedulerHandlerForEnv(env *models.Env) apiv2.Handler {
	return apiv2.New(f.kubeUtil, env)
}

// NewFactory Constructor for factory
func NewFactory(kubeUtil *kube.Kube) Interface {
	return &factory{kubeUtil}
}
