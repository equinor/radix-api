package models

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
	// Memory Requests
	// required: true
	MemReqs float64 `json:"mem_reqs"`
	// Max memory used
	// required: true
	MemMax float64 `json:"mem_max"`
	// Cpu Requests
	// required: true
	CpuReqs float64 `json:"cpu_reqs"`
	// Average CPU Used
	// required: true
	CpuAvg float64 `json:"cpu_avg"`
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
