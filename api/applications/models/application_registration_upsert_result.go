package models

// ApplicationRegistrationUpsertResult describe an application upsert operation result
// swagger:model ApplicationRegistrationUpsertResult
type ApplicationRegistrationUpsertResult struct {
	// ApplicationRegistration
	//
	// required: true
	ApplicationRegistration *ApplicationRegistration `json:"applicationRegistration"`

	// Warnings of upsert operation
	//
	// required: false
	// example: ["Repository is in use by App1"]
	Warnings []string `json:"warnings,omitempty"`
}
