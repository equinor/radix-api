package models

// JobParameters parameters to create a pipeline job
// Not exposed in the API
type JobParameters struct {
	// Name of the branch
	Branch string `json:"branch"`

	// Commit ID of the branch
	CommitID string `json:"commitID"`

	// should image be pushed to container registry
	PushImage bool `json:"pushImage"`
}

func (param JobParameters) GetPushImageTag() string {
	if param.PushImage {
		return "1"
	} else {
		return "0"
	}
}
