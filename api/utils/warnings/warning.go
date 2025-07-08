package warnings

import (
	"k8s.io/client-go/rest"
)

// type WarningHandler interface {
// 	// HandleWarningHeader is called with the warn code, agent, and text when a warning header is countered.
// 	HandleWarningHeader(code int, agent string, text string)
// }

type CollectWarnings struct {
	// Warnings is a slice of warning messages.
	Warnings []string `json:"warnings,omitempty"`
}

var _ rest.WarningHandler = &CollectWarnings{}

func New() *CollectWarnings {
	return &CollectWarnings{
		Warnings: make([]string, 0),
	}
}

func (c *CollectWarnings) HandleWarningHeader(code int, agent string, text string) {
	if text != "" {
		c.Warnings = append(c.Warnings, text)
	}
}
