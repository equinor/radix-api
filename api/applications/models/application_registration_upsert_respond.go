package models

// ApplicationRegistrationUpsertRespond describe an application upsert operation respond
// swagger:model ApplicationRegistrationUpsertRespond
type ApplicationRegistrationUpsertRespond struct {
	// ApplicationRegistration
	//
	// required: false
	ApplicationRegistration *ApplicationRegistration `json:"applicationRegistration"`

	// Warnings of upsert operation
	//
	// required: false
	// example: ["Repository is in use by App1"]
	Warnings []string `json:"warnings,omitempty"`
}
