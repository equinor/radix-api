package jobs

// PipelineJob hold info about pipeline job
// todo: only need appname and branch?
// swagger:model PipelineJob
type PipelineJob struct {
	// Name of the job
	//
	// required: false
	Name string `json:"name"`

	// Name of the application
	//
	// required: true
	AppName string `json:"appname"`

	// Name of the branch
	//
	// required: true
	Branch string `json:"branch"`

	// Commit ID of the branch
	//
	// required: false
	CommitID string `json:"commitID"`

	// Refers to the repo of the app this job is for
	//
	// required: false
	SSHRepo string `json:"sshRepo"`

	// Status of the job
	//
	// required: false
	JobStatus string `json:"jobStatus"`
}
