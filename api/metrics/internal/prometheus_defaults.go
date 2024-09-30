package internal

// QueryName Prometheus query name
type QueryName string

const (
	CpuMax    QueryName = "CpuMax"
	CpuMin    QueryName = "CpuMin"
	CpuAvg    QueryName = "CpuAvg"
	MemoryMax QueryName = "MemoryMax"
	MemoryMin QueryName = "MemoryMin"
	MemoryAvg QueryName = "MemoryAvg"
)
