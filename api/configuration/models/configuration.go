package models

// Environment holds detail information about environment
// swagger:model ConfigurationSettings
type Settings struct {
	ClusterEgressIps   []string `json:"clusterEgressIps"`
	ClusterOidcIssuers []string `json:"clusterOidcIssuers"`
	ClusterBaseDomain  string   `json:"clusterBaseDomain"`
	ClusterType        string   `json:"clusterType"`
	ClusterName        string   `json:"clusterName"`
}
