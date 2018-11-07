package models

// Pod hold info about pod
// swagger:model Pod
type Pod struct {
	// Name of the pod
	//
	// required: true
	Name string `json:"name"`
}
