package jobs

// PipelineJob hold info about pipeline job
// todo: only need appname and branch?
// swagger:model PipelineJob
type PipelineJob struct {
	// Name of the job
	//
	// required: true
	Name string `json:"name"`

	// Name of the branch
	//
	// required: true
	Branch string `json:"branch"`

	// Name of the branch
	//
	// required: true
	SSHRepo string `json:"sshRepo"`
}
