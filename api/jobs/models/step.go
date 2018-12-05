package models

// Step holds general information about job step
// swagger:model Step
type Step struct {
	// Name of the step
	//
	// required: false
	// example: build
	Name string `json:"name"`

	// Status of the step
	//
	// required: false
	// Enum: Waiting,Running,Succeeded,Failed
	// example: Waiting
	Status string `json:"status"`

	// Started timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Started string `json:"started"`

	// Ended timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Ended string `json:"ended"`

	// Pod name
	//
	// required: false
	PodName string `json:"-"`

	// sort steps
	//
	// required: false
	Sort int32 `json:"-"`
}
