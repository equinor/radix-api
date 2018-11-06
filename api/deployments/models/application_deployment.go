package models

// ApplicationDeployment describe an deployment
// swagger:model ApplicationDeployment
type ApplicationDeployment struct {
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

	// Created Created timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05-0700
	Created string `json:"created"`
}
