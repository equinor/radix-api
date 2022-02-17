package models

// Deployment describe an deployment
// swagger:model Deployment
type Deployment struct {
	// Name the unique name of the Radix application deployment
	//
	// required: false
	// example: radix-canary-golang-tzbqi
	Name string `json:"name"`

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
	// required: false
	// example: 2006-01-02T15:04:05Z
	ActiveFrom string `json:"activeFrom"`

	// ActiveTo Timestamp when the deployment ends
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	ActiveTo string `json:"activeTo,omitempty"`
}

func (d *Deployment) GetComponentByName(name string) *Component {
	for _, c := range d.Components {
		if c != nil && c.Name == name {
			return c
		}
	}

	return nil
}
