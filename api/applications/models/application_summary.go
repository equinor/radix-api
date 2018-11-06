package models

// ApplicationSummary describe an application
// swagger:model ApplicationSummary
type ApplicationSummary struct {
	// Name the name of the application
	//
	// required: false
	// example: radix-canary-golang
	Name string `json:"name"`
}
