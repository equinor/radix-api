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
	WithCount(int32) EventBuilder
	WithObjectKind(string) EventBuilder
	WithObjectNamespace(string) EventBuilder
	WithObjectName(string) EventBuilder
	WithType(string) EventBuilder
	WithReason(string) EventBuilder
	WithMessage(string) EventBuilder
	BuildEvent() *Event
}

type eventBuilder struct {
	lastTimestamp   time.Time
	count           int32
	objectKind      string
	objectNamespace string
	objectName      string
	eventType       string
	reason          string
	message         string
}

// NewEventBuilder Constructor for eventBuilder
func NewEventBuilder() EventBuilder {
	return &eventBuilder{}
}

func (eb *eventBuilder) WithKubernetesEvent(v v1.Event) EventBuilder {
	eb.WithLastTimestamp(v.LastTimestamp.Time)
	eb.WithCount(v.Count)
	eb.WithObjectKind(v.InvolvedObject.Kind)
	eb.WithObjectNamespace(v.InvolvedObject.Namespace)
	eb.WithObjectName(v.InvolvedObject.Name)
	eb.WithType(v.Type)
	eb.WithReason(v.Reason)
	eb.WithMessage(v.Message)
	return eb
}

func (eb *eventBuilder) WithLastTimestamp(v time.Time) EventBuilder {
	eb.lastTimestamp = v
	return eb
}

func (eb *eventBuilder) WithCount(v int32) EventBuilder {
	eb.count = v
	return eb
}

func (eb *eventBuilder) WithObjectKind(v string) EventBuilder {
	eb.objectKind = v
	return eb
}

func (eb *eventBuilder) WithObjectNamespace(v string) EventBuilder {
	eb.objectNamespace = v
	return eb
}

func (eb *eventBuilder) WithObjectName(v string) EventBuilder {
	eb.objectName = v
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
		LastTimestamp:   strfmt.DateTime(eb.lastTimestamp),
		Count:           eb.count,
		ObjectKind:      eb.objectKind,
		ObjectNamespace: eb.objectNamespace,
		ObjectName:      eb.objectName,
		Type:            eb.eventType,
		Reason:          eb.reason,
		Message:         eb.message,
	}
}
