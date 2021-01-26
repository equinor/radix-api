package models

// BuildStatus describe an deployment
// swagger:model BuildStatus
type BuildStatus struct {
	// Name the unique name of the Radix application build status
	//
	// required: false
	// example: radix-canary-golang-tzbqi
	Name string `json:"name"`

	// Environment the environment this Radix application deployment runs in
	//
	// required: false
	// example: prod
	Environment string `json:"environment"`

	// Status
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Status string `json:"status"`
}
