package models

// JobParameters parameters to create a pipeline job
// Not exposed in the API
type JobParameters struct {
	// Name of the branch
	Branch string `json:"branch"`

	// Commit ID of the branch
	CommitID string `json:"commitID"`
}
