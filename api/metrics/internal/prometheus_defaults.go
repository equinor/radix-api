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

var Queries = map[QueryName]string{
	CpuRequests:   `max by(namespace, container, pod) (kube_pod_container_resource_requests{container!="",namespace!="%s-app", namespace=~"%s-%s",resource="cpu"}) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component!=""}`,
	MemoryRequest: `max by(namespace, container, pod) (kube_pod_container_resource_requests{container!="",namespace!="%s-app", namespace=~"%s-%s",resource="memory"}) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component!=""}`,
	CpuMax:        `max by(namespace, container, pod) (max_over_time(rate(container_cpu_usage_seconds_total{container!="",namespace!="%s-app", namespace=~"%s-%s"}[1m]) [%s:1m])) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component!=""}`,
	MemoryMax:     `max by(namespace, container, pod) (max_over_time(container_memory_usage_bytes{container!="",namespace!="%s-app", namespace=~"%s-%s"} [%s:1m])) * on(pod) group_left(label_radix_component) kube_pod_labels{label_radix_component!=""}`,
}
