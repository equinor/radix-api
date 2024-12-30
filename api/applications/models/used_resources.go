package models

import "math"

// UsedResources holds information about used resources
// swagger:model UsedResources
type UsedResources struct {
	// From timestamp
	//
	// required: true
	// example: 2006-01-02T15:04:05Z
	From string `json:"from"`

	// To timestamp
	//
	// required: true
	// example: 2006-01-03T15:04:05Z
	To string `json:"to"`

	// CPU used, in cores
	//
	// required: false
	CPU *UsedResource `json:"cpu,omitempty"`

	// Memory used, in bytes
	//
	// required: false
	Memory *UsedResource `json:"memory,omitempty"`

	// Warning messages
	//
	// required: false
	Warnings []string `json:"warnings,omitempty"`
}

// UsedResource holds information about used resource
// swagger:model UsedResource
type UsedResource struct {
	// Min resource used
	//
	// required: false
	// example: 0.00012
	Min *float64 `json:"min,omitempty"`

	// Avg Average resource used
	//
	// required: false
	// example: 0.00023
	Avg *float64 `json:"avg,omitempty"`

	// Max resource used
	//
	// required: false
	// example: 0.00037
	Max *float64 `json:"max,omitempty"`
}

// ReplicaResourcesUtilizationResponse holds information about resource utilization
// swagger:model ReplicaResourcesUtilizationResponse
type ReplicaResourcesUtilizationResponse struct {
	Environments map[string]EnvironmentUtilization `json:"environments"`
}

type EnvironmentUtilization struct {
	Components map[string]ComponentUtilization `json:"components"`
}

type ComponentUtilization struct {
	RequestedCPU    float64                       `json:"requested_cpu"`
	RequestedMemory float64                       `json:"requested_memory"`
	Replica         map[string]ReplicaUtilization `json:"replicas"`
}
type ReplicaUtilization struct {
	MaxMemory float64 `json:"max_memory"`
	MaxCPU    float64 `json:"max_cpu"`
}

func NewPodResourcesUtilizationResponse() *ReplicaResourcesUtilizationResponse {
	return &ReplicaResourcesUtilizationResponse{
		Environments: make(map[string]EnvironmentUtilization),
	}
}

func (r *ReplicaResourcesUtilizationResponse) SetCpuRequests(environment, component string, value float64) {
	r.ensureComponent(environment, component)

	c := r.Environments[environment].Components[component]
	c.RequestedCPU = math.Round(value*1e6) / 1e6
	r.Environments[environment].Components[component] = c
}

func (r *ReplicaResourcesUtilizationResponse) SetMemoryRequests(environment, component string, value float64) {
	r.ensureComponent(environment, component)

	c := r.Environments[environment].Components[component]
	c.RequestedMemory = math.Round(value)
	r.Environments[environment].Components[component] = c
}

func (r *ReplicaResourcesUtilizationResponse) SetMaxCpuUsage(environment, component, pod string, value float64) {
	r.ensurePod(environment, component, pod)

	p := r.Environments[environment].Components[component].Replica[pod]
	p.MaxCPU = math.Round(value*1e6) / 1e6
	r.Environments[environment].Components[component].Replica[pod] = p
}

func (r *ReplicaResourcesUtilizationResponse) SetMaxMemoryUsage(environment, component, pod string, value float64) {
	r.ensurePod(environment, component, pod)

	p := r.Environments[environment].Components[component].Replica[pod]
	p.MaxMemory = math.Round(value)
	r.Environments[environment].Components[component].Replica[pod] = p
}

func (r *ReplicaResourcesUtilizationResponse) ensureComponent(environment, component string) {
	if _, ok := r.Environments[environment]; !ok {
		r.Environments[environment] = EnvironmentUtilization{
			Components: make(map[string]ComponentUtilization),
		}
	}

	if _, ok := r.Environments[environment].Components[component]; !ok {
		r.Environments[environment].Components[component] = ComponentUtilization{
			Replica: make(map[string]ReplicaUtilization),
		}
	}

}

func (r *ReplicaResourcesUtilizationResponse) ensurePod(environment, component, pod string) {
	r.ensureComponent(environment, component)

	if _, ok := r.Environments[environment].Components[component].Replica[pod]; !ok {
		r.Environments[environment].Components[component].Replica[pod] = ReplicaUtilization{}
	}
}
