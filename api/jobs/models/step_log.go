package models

// StepLog holds logs for a given step
// swagger:model StepLog
type StepLog struct {
	// Name of the step
	//
	// required: true
	Name string `json:"name"`

	// Name of the logged pod
	//
	// required: false
	PodName string `json:"podname"`

	// Log of step
	Log string `json:"log"`

	Sort int32 `json:"sort"`
}
