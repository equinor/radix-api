package deployment

import (
	"time"
)

// ApplicationDeployment describe an deployment
// swagger:model ApplicationDeployment
type ApplicationDeployment struct {
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

	// Name the unique name of the Radix application deployment
	//
	// required: false
	// example: radix-canary-golang-tzbqi
	Name string `json:"name"`

	// Created Created timestamp
	//
	// required: false
	// example: timestamp
	Created time.Time `json:"created"`
}
