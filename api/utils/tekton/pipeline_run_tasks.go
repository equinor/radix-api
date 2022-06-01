package tekton

import (
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

//GetTaskRealNameToNameMap Get Tekton Task real name to name map for the Radix pipeline run
func GetTaskRealNameToNameMap(pipelineRun *v1beta1.PipelineRun) map[string]string {
	nameMap := make(map[string]string)
	if pipelineRun.Status.PipelineSpec == nil {
		return nameMap
	}
	for _, task := range pipelineRun.Status.PipelineSpec.Tasks {
		if task.TaskRef != nil {
			nameMap[task.TaskRef.Name] = task.Name
		}
	}
	return nameMap
}
