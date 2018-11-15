package models

// LogMsg represent a single line of log message
// swagger:model LogMsg
type LogMsg struct {
	// Aggregation of log. This could be step name, but also pod name
	//
	// required: true
	Name string `json:"name"`

	// Message appended to log
	//
	// required: true
	AppendLog string `json:"appendLog"`
}
