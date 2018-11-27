package models

import (
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
)

// ComponentDeployment describe an component part of an deployment
// swagger:model ComponentDeployment
type ComponentDeployment struct {
	// Name the component
	//
	// required: true
	// example: server
	Name string `json:"name"`

	// Image name
	//
	// required: true
	// example: radixdev.azurecr.io/radix-api-server:cdgkg
	Image string `json:"image"`

	// ComponentPort defines the port number, protocol and port for a service
	//
	// required: false
	Ports []v1.ComponentPort `json:"ports"`

	// Secret names that will be mapped from the environment
	//
	// required: false
	Secrets []string `json:"secrets"`

	// Variable names map to values)
	//
	// required: false
	Variables map[string]string `json:"variables"`

	// Array of pod names
	//
	// required: false
	Replicas []string `json:"replicas"`
}

// ComponentSummary describe an component part of an deployment
// swagger:model ComponentSummary
type ComponentSummary struct {
	// Name the component
	//
	// required: true
	// example: server
	Name string `json:"name"`

	// Image name
	//
	// required: true
	// example: radixdev.azurecr.io/radix-api-server:cdgkg
	Image string `json:"image"`
}
