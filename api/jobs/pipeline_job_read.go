package jobs

import (
	batchv1 "k8s.io/api/batch/v1"
)

// PipelineJobRead hold info about pipeline job
// swagger:model PipelineJob
type PipelineJobRead struct {
	// Name of the job
	//
	// required: true
	Name string `json:"name"`
	// Name of the application
	//
	// required: true
	AppName string `json:"appname"`

	// Name of the branch
	//
	// required: true
	Branch string `json:"branch"`

	// Github commit id
	//
	// required: false
	CommitID string `json:"commitID"`

	// job type (pipeline or build)
	//
	// required: false
	Type string `json:"type"`

	// event type (added, updated or deleted)
	//
	// required: false
	Event string `json:"event"`

	Status batchv1.JobStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}
