package tekton

import (
	"context"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeLabels "k8s.io/apimachinery/pkg/labels"
)

// GetTaskRealNameToNameMap Get Tekton Task real name to name map for the Radix pipeline run
func GetTaskRealNameToNameMap(pipelineRun *pipelinev1.PipelineRun) map[string]string {
	if pipelineRun.Status.PipelineSpec == nil {
		return make(map[string]string)
	}
	return slice.Reduce(pipelineRun.Status.PipelineSpec.Tasks, make(map[string]string), func(acc map[string]string, task pipelinev1.PipelineTask) map[string]string {
		if task.TaskRef != nil {
			acc[task.TaskRef.Name] = task.Name
		}
		return acc
	})
}

// GetTektonPipelineTaskRuns Get Tekton TaskRuns for the Radix pipeline job
func GetTektonPipelineTaskRuns(ctx context.Context, tektonClient tektonclient.Interface, appName, jobName string, pipelineRunName string) (map[string]*pipelinev1.TaskRun, error) {
	namespace := crdUtils.GetAppNamespace(appName)
	taskRunList, err := tektonClient.TektonV1().TaskRuns(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: kubeLabels.Set{
			kube.RadixJobNameLabel:   jobName,
			"tekton.dev/pipelineRun": pipelineRunName,
		}.String(),
	})
	if err != nil {
		return nil, err
	}
	return slice.Reduce(taskRunList.Items, make(map[string]*pipelinev1.TaskRun), func(acc map[string]*pipelinev1.TaskRun, taskRun pipelinev1.TaskRun) map[string]*pipelinev1.TaskRun {
		if taskRun.Spec.TaskRef != nil {
			acc[taskRun.Spec.TaskRef.Name] = &taskRun
		}
		return acc
	}), nil
}
