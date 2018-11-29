package models

// DeploymentSummary describe an deployment
// swagger:model DeploymentSummary
type DeploymentSummary struct {
	// Name the unique name of the Radix application deployment
	//
	// required: false
	// example: radix-canary-golang-tzbqi
	Name string `json:"name"`

	// AppName the name of the Radix application owning this deployment
	//
	// required: false
	// example: radix-canary-golang
	AppName string `json:"appName"`

	// Environment the environment this Radix application deployment runs in
	//
	// required: false
	// example: prod
	Environment string `json:"environment"`

	// ActiveFrom Timestamp when the deployment starts (or created)
	//
	// required: false
	// example: 2006-01-02T15:04:05-0700
	ActiveFrom string `json:"activeFrom"`

	// ActiveTo Timestamp when the deployment ends
	//
	// required: false
	// example: 2006-01-02T15:04:05-0700
	ActiveTo string `json:"activeTo,omitempty"`

	// Array of components
	//
	// required: false
	// items:
	//    "$ref": "#/definitions/ComponentSummary"
	Components []*ComponentSummary `json:"-"`

	// Name of job creating deployment
	//
	// required: false
	JobName string `json:"-"`
}
