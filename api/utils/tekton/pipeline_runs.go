package tekton

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	crdUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeLabels "k8s.io/apimachinery/pkg/labels"
)

//GetTektonPipelineRuns Get Tekton PipelineRuns for the Radix pipeline job
func GetTektonPipelineRuns(tektonClient tektonclient.Interface, appName, jobName string) ([]v1beta1.PipelineRun, error) {
	namespace := crdUtils.GetAppNamespace(appName)
	pipelineRunList, err := tektonClient.TektonV1beta1().PipelineRuns(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: kubeLabels.Set{
			kube.RadixJobNameLabel: jobName,
		}.String(),
	})
	return pipelineRunList.Items, err
}

//GetPipelineRun Get Tekton PipelineRun for the Radix pipeline job
func GetPipelineRun(tektonClient tektonclient.Interface, appName, jobName, pipelineRunName string) (*v1beta1.PipelineRun, error) {
	namespace := crdUtils.GetAppNamespace(appName)
	pipelineRun, err := tektonClient.TektonV1beta1().PipelineRuns(namespace).Get(context.TODO(), pipelineRunName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if pipelineRun.ObjectMeta.Labels[kube.RadixJobNameLabel] != jobName {
		return nil, fmt.Errorf("pipeline run %s belongs to different pipeline job than requested %s", pipelineRunName, jobName)
	}
	return pipelineRun, nil
}
