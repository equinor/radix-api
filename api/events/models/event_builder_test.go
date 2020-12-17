package models

import (
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_EventBuilder_FluentApi_SingleField(t *testing.T) {

	t.Run("WithLastTimestamp", func(t *testing.T) {
		v := time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC)
		e := NewEventBuilder().
			WithLastTimestamp(v).
			BuildEvent()
		assert.Equal(t, strfmt.DateTime(v), e.LastTimestamp)
	})

	t.Run("WithMessage", func(t *testing.T) {
		v := "msg"
		e := NewEventBuilder().
			WithMessage(v).
			BuildEvent()
		assert.Equal(t, v, e.Message)
	})

	t.Run("WithObjectKind", func(t *testing.T) {
		v := "kind"
		e := NewEventBuilder().
			WithInvolvedObjectKind(v).
			BuildEvent()
		assert.Equal(t, v, e.InvolvedObjectKind)
	})

	t.Run("WithObjectName", func(t *testing.T) {
		v := "name"
		e := NewEventBuilder().
			WithInvolvedObjectName(v).
			BuildEvent()
		assert.Equal(t, v, e.InvolvedObjectName)
	})

	t.Run("WithObjectNamespace", func(t *testing.T) {
		v := "ns"
		e := NewEventBuilder().
			WithInvolvedObjectNamespace(v).
			BuildEvent()
		assert.Equal(t, v, e.InvolvedObjectNamespace)
	})

	t.Run("WithReason", func(t *testing.T) {
		v := "reason"
		e := NewEventBuilder().
			WithReason(v).
			BuildEvent()
		assert.Equal(t, v, e.Reason)
	})

	t.Run("WithType", func(t *testing.T) {
		v := "type"
		e := NewEventBuilder().
			WithType(v).
			BuildEvent()
		assert.Equal(t, v, e.Type)
	})
}

func Test_EventBuilder_FluentApi_WithKubernetes(t *testing.T) {
	lastTs := time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC)
	v := v1.Event{
		LastTimestamp: metav1.NewTime(lastTs),
		Message:       "msg",
		Type:          "type",
		Reason:        "reason",
		InvolvedObject: v1.ObjectReference{
			Kind:      "kind",
			Name:      "name",
			Namespace: "ns",
		},
	}

	e := NewEventBuilder().
		WithKubernetesEvent(v).
		BuildEvent()

	assert.Equal(t, strfmt.DateTime(lastTs), e.LastTimestamp)
	assert.Equal(t, v.Message, e.Message)
	assert.Equal(t, v.InvolvedObject.Kind, e.InvolvedObjectKind)
	assert.Equal(t, v.InvolvedObject.Name, e.InvolvedObjectName)
	assert.Equal(t, v.InvolvedObject.Namespace, e.InvolvedObjectNamespace)
	assert.Equal(t, v.Reason, e.Reason)
	assert.Equal(t, v.Type, e.Type)
}
