package internal

import "regexp"

// QueryName Prometheus query name
type QueryName string

const (
	CpuMax        QueryName = "CpuMax"
	CpuMin        QueryName = "CpuMin"
	CpuAvg        QueryName = "CpuAvg"
	CpuRequests   QueryName = "SetCpuRequests"
	MemoryMax     QueryName = "MemoryMax"
	MemoryMin     QueryName = "MemoryMin"
	MemoryAvg     QueryName = "MemoryAvg"
	MemoryRequest QueryName = "MemoryRequest"
)

const (
	DefaultDuration = "30d"
	DefaultOffset   = ""
)

var DurationExpression = regexp.MustCompile(`^[0-9]{1,5}[mhdw]$`)
