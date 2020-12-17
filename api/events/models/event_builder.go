package models

import (
	"time"

	"github.com/go-openapi/strfmt"
	v1 "k8s.io/api/core/v1"
)

// EventBuilder Build Event DTOs
type EventBuilder interface {
	WithKubernetesEvent(v1.Event) EventBuilder
	WithLastTimestamp(time.Time) EventBuilder
	WithInvolvedObjectKind(string) EventBuilder
	WithInvolvedObjectNamespace(string) EventBuilder
	WithInvolvedObjectName(string) EventBuilder
	WithType(string) EventBuilder
	WithReason(string) EventBuilder
	WithMessage(string) EventBuilder
	BuildEvent() *Event
}

type eventBuilder struct {
	lastTimestamp           time.Time
	involvedObjectKind      string
	involvedObjectNamespace string
	involvedObjectName      string
	eventType               string
	reason                  string
	message                 string
}

// NewEventBuilder Constructor for eventBuilder
func NewEventBuilder() EventBuilder {
	return &eventBuilder{}
}

func (eb *eventBuilder) WithKubernetesEvent(v v1.Event) EventBuilder {
	eb.WithLastTimestamp(v.LastTimestamp.Time)
	eb.WithInvolvedObjectKind(v.InvolvedObject.Kind)
	eb.WithInvolvedObjectNamespace(v.InvolvedObject.Namespace)
	eb.WithInvolvedObjectName(v.InvolvedObject.Name)
	eb.WithType(v.Type)
	eb.WithReason(v.Reason)
	eb.WithMessage(v.Message)
	return eb
}

func (eb *eventBuilder) WithLastTimestamp(v time.Time) EventBuilder {
	eb.lastTimestamp = v
	return eb
}

func (eb *eventBuilder) WithInvolvedObjectKind(v string) EventBuilder {
	eb.involvedObjectKind = v
	return eb
}

func (eb *eventBuilder) WithInvolvedObjectNamespace(v string) EventBuilder {
	eb.involvedObjectNamespace = v
	return eb
}

func (eb *eventBuilder) WithInvolvedObjectName(v string) EventBuilder {
	eb.involvedObjectName = v
	return eb
}

func (eb *eventBuilder) WithType(v string) EventBuilder {
	eb.eventType = v
	return eb
}

func (eb *eventBuilder) WithReason(v string) EventBuilder {
	eb.reason = v
	return eb
}

func (eb *eventBuilder) WithMessage(v string) EventBuilder {
	eb.message = v
	return eb
}

func (eb *eventBuilder) BuildEvent() *Event {
	return &Event{
		LastTimestamp:           strfmt.DateTime(eb.lastTimestamp),
		InvolvedObjectKind:      eb.involvedObjectKind,
		InvolvedObjectNamespace: eb.involvedObjectNamespace,
		InvolvedObjectName:      eb.involvedObjectName,
		Type:                    eb.eventType,
		Reason:                  eb.reason,
		Message:                 eb.message,
	}
}
