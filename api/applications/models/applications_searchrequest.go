package models

// ApplicationsSearchRequest contains the list of application names to be queried
// swagger:model ApplicationsSearchRequest
type ApplicationsSearchRequest struct {
	// List of application names to be returned
	//
	// required: true
	// example: ["app1", "app2"]
	Names []string `json:"names"`

	// List of application names to be returned
	//
	// required: false
	// example: { jobSummary: true }
	IncludeFields ApplicationSearchIncludeFields `json:"includeFields,omitempty"`
}

// ApplicationSearchIncludeFields specifies additional fields to include in the response of an ApplicationsSearchRequest
// swagger:model ApplicationSearchIncludeFields
type ApplicationSearchIncludeFields struct {
	LatestJobSummary            bool `json:"latestJobSummary,omitempty"`
	EnvironmentActiveComponents bool `json:"environmentActiveComponents,omitempty"`
}
