package models

// Component describe an component part of an deployment
// swagger:model Component
type Component struct {
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

	// Ports defines the port number and protocol that a component is exposed for internally in environment
	//
	// required: false
	// type: "array"
	// items:
	//    "$ref": "#/definitions/Port"
	Ports []Port `json:"ports"`

	// Component secret names. From radixconfig.yaml
	//
	// required: false
	// example: DB_CON,A_SECRET
	Secrets []string `json:"secrets"`

	// Variable names map to values. From radixconfig.yaml
	//
	// required: false
	Variables map[string]string `json:"variables"`

	// Array of ReplicaSummary
	//
	// required: false
	Replicas []ReplicaSummary `json:"replicas"`
}

// Port describe an component part of an deployment
// swagger:model Port
type Port struct {
	// Component port name. From radixconfig.yaml
	//
	// required: true
	// example: http
	Name string `json:"name"`

	// Component port number. From radixconfig.yaml
	//
	// required: false
	// example: 8080
	Port int32 `json:"port"`
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

// ReplicaSummary describes condition of a pod
// swagger:model ReplicaSummary
type ReplicaSummary struct {
	// Pod name
	//
	// required: true
	// example: server-78fc8857c4-hm76l
	Name string `json:"name"`

	// Status describes the component container status
	//
	// required: false
	Status ReplicaStatus `json:"replicaStatus"`

	// StatusMessage provides message describing the status of a component container inside a pod
	//
	// required: false
	StatusMessage string `json:"statusMessage"`
}

// ReplicaStatus describes the status of a component container inside a pod
type ReplicaStatus struct {
	// Status of the container
	// - Pending = Container in Waiting state and the reason is ContainerCreating
	// - Failing = Container in Waiting state and the reason is anything else but ContainerCreating
	// - Running = Container in Running state
	// - Terminated = Container in Terminated state
	//
	// required: true
	// example: Pending, Failing, Running, Terminated
	Status string `json:"status"`
}
