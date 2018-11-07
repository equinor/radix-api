package models

// Job holds general information about job
// swagger:model Job
type Job struct {
	// Name of the job
	//
	// required: false
	// example: radix-pipeline-20181029135644-algpv-6hznh
	Name string `json:"name"`

	// TriggeredBy who triggered the job
	//
	// required: false
	// example: radix-pipeline-20181029135644-algpv-6hznh
	TriggeredBy string `json:"triggeredBy"`

	// Started timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05-0700
	Started string `json:"started"`

	// Ended timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05-0700
	Ended string `json:"ended"`

	// Status of the job
	//
	// required: false
	// Enum: [Pending, Running, Success, Fail]
	// example: Pending
	Status string `json:"status"`

	// Name of the pipeline
	//
	// required: false
	// Enum: [build-deploy]
	// example: build-deploy
	Pipeline string `json:"pipeline"`

	// List of steps
	//
	// required: false
	// type: "array"
	// items:
	//    "$ref": "#/definitions/Step"
	Steps []Step `json:"steps"`
}
