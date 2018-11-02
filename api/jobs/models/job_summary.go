package models

// JobSummary holds general information about job
// swagger:model JobSummary
type JobSummary struct {
	// Name of the job
	//
	// required: false
	// example: radix-pipeline-20181029135644-algpv-6hznh
	Name string `json:"name"`

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
	Status string `json:"status"`
}
