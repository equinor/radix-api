package jobs

// PipelineJob hold info about pipeline job
// todo: only need appname and branch?
// swagger:model PipelineJob
type PipelineJob struct {
	// Name of the job
	//
	// required: false
	Name string `json:"name"`

	// Name of the branch
	//
	// required: true
	Branch string `json:"branch"`

	// Commit ID of the branch
	//
	// required: true
	CommitID string `json:"commitID"`

	// Name of the branch
	//
	// required: true
	SSHRepo string `json:"sshRepo"`
}
