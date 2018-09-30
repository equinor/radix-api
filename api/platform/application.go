package platform

// Application describe an application
type Application struct {
	// Repository the github repository
	//
	// required: false
	Name string `json:"name"`

	// Repository the github repository
	//
	// required: false
	Repository string `json:"repository"`

	// Description the status of the application
	//
	// required: false
	Description string `json:"description"`
}
