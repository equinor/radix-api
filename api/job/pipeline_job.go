package job

// PipelineJob hold info about pipeline job
// swagger:model pipelineJob
type PipelineJob struct {
	// Name of the job
	//
	// required: false
	Name string `json:"name"`

	// Name of the application
	//
	// required: false
	AppName string `json:"appName"`

	// Name of the branch
	//
	// required: false
	Branch string `json:"branch"`

	// Name of the branch
	//
	// required: false
	SSHRepo string `json:"sshRepo"`
}

// PipelineJobsResponse hold info about pipeline job
// swagger:response pipelineJobsResp
type PipelineJobsResponse struct {
	Jobs []PipelineJob
}
