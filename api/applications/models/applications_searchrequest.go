package models

// ApplicationsSearchRequest contains the list of application names to be queried
// swagger:model ApplicationsSearchRequest
type ApplicationsSearchRequest struct {
	// List of application names to be returned
	//
	// required: true
	// example: ["app1", "app2"]
	Names []string `json:"names"`
}
