package models

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
	Replicas map[string]ReplicaUtilization `json:"replicas"`
}
type ReplicaUtilization struct {
	MemReqs float64 `json:"mem_reqs"`
	MemMax  float64 `json:"mem_max"`
	CpuReqs float64 `json:"cpu_reqs"`
	CpuAvg  float64 `json:"cpu_avg"`
}

func NewPodResourcesUtilizationResponse() *ReplicaResourcesUtilizationResponse {
	return &ReplicaResourcesUtilizationResponse{
		Environments: make(map[string]EnvironmentUtilization),
	}
}

func (r *ReplicaResourcesUtilizationResponse) SetCpuReqs(environment, component, pod string, value float64) {
	r.ensurePod(environment, component, pod)

	p := r.Environments[environment].Components[component].Replicas[pod]
	p.CpuReqs = value
	r.Environments[environment].Components[component].Replicas[pod] = p
}

func (r *ReplicaResourcesUtilizationResponse) SetCpuAvg(environment, component, pod string, value float64) {
	r.ensurePod(environment, component, pod)

	p := r.Environments[environment].Components[component].Replicas[pod]
	p.CpuAvg = value
	r.Environments[environment].Components[component].Replicas[pod] = p
}

func (r *ReplicaResourcesUtilizationResponse) SetMemReqs(environment, component, pod string, value float64) {
	r.ensurePod(environment, component, pod)

	p := r.Environments[environment].Components[component].Replicas[pod]
	p.MemReqs = value
	r.Environments[environment].Components[component].Replicas[pod] = p
}

func (r *ReplicaResourcesUtilizationResponse) SetMemMax(environment, component, pod string, value float64) {
	r.ensurePod(environment, component, pod)

	p := r.Environments[environment].Components[component].Replicas[pod]
	p.MemMax = value
	r.Environments[environment].Components[component].Replicas[pod] = p
}

func (r *ReplicaResourcesUtilizationResponse) ensurePod(environment, component, pod string) {
	if _, ok := r.Environments[environment]; !ok {
		r.Environments[environment] = EnvironmentUtilization{
			Components: make(map[string]ComponentUtilization),
		}
	}

	if _, ok := r.Environments[environment].Components[component]; !ok {
		r.Environments[environment].Components[component] = ComponentUtilization{
			Replicas: make(map[string]ReplicaUtilization),
		}
	}

	if _, ok := r.Environments[environment].Components[component].Replicas[pod]; !ok {
		r.Environments[environment].Components[component].Replicas[pod] = ReplicaUtilization{}
	}
}
