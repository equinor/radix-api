package models

// EnvVarParameters describes an environment variable
// swagger:model EnvVarParameters
type EnvVarParameters struct {
	// Value a new value of the environment variable
	//
	// required: true
	// example: value1
	Value string `json:"value"`
}
