package models

import "time"

// Deployment describe an deployment
// swagger:model Deployment
type Deployment struct {
	// Name the unique name of the Radix application deployment
	//
	// required: true
	// example: radix-canary-golang-tzbqi
	Name string `json:"name"`

	// Namespace where the deployment is stored
	//
	// required: true
	// example: radix-canary-golang-dev
	Namespace string `json:"namespace"`

	// Array of components
	//
	// required: false
	Components []*Component `json:"components,omitempty"`

	// Name of job creating deployment
	//
	// required: false
	CreatedByJob string `json:"createdByJob,omitempty"`

	// Environment the environment this Radix application deployment runs in
	//
	// required: false
	// example: prod
	Environment string `json:"environment"`

	// ActiveFrom Timestamp when the deployment starts (or created)
	//
	// required: true
	// swagger:strfmt date-time
	ActiveFrom time.Time `json:"activeFrom"`

	// ActiveTo Timestamp when the deployment ends
	//
	// required: false
	// swagger:strfmt date-time
	ActiveTo *time.Time `json:"activeTo"`

	// GitCommitHash the hash of the git commit from which radixconfig.yaml was parsed
	//
	// required: false
	// example: 4faca8595c5283a9d0f17a623b9255a0d9866a2e
	GitCommitHash string `json:"gitCommitHash,omitempty"`

	// GitTags the git tags that the git commit hash points to
	//
	// required: false
	// example: "v1.22.1 v1.22.3"
	GitTags string `json:"gitTags,omitempty"`

	// Repository the GitHub repository that the deployment was built from
	//
	// required: true
	// example: https://github.com/equinor/radix-canary-golang
	Repository string `json:"repository,omitempty"`

	// Name of the branch used to build the deployment
	//
	// required: false
	// example: main
	BuiltFromBranch string `json:"builtFromBranch,omitempty"`
}

func (d *Deployment) GetComponentByName(name string) *Component {
	for _, c := range d.Components {
		if c != nil && c.Name == name {
			return c
		}
	}

	return nil
}
