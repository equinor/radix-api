// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/events/event_handler.go

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	events "github.com/equinor/radix-api/api/events"
	models "github.com/equinor/radix-api/api/events/models"
	gomock "github.com/golang/mock/gomock"
)

// MockEventHandler is a mock of EventHandler interface.
type MockEventHandler struct {
	ctrl     *gomock.Controller
	recorder *MockEventHandlerMockRecorder
}

// MockEventHandlerMockRecorder is the mock recorder for MockEventHandler.
type MockEventHandlerMockRecorder struct {
	mock *MockEventHandler
}

// NewMockEventHandler creates a new mock instance.
func NewMockEventHandler(ctrl *gomock.Controller) *MockEventHandler {
	mock := &MockEventHandler{ctrl: ctrl}
	mock.recorder = &MockEventHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockEventHandler) EXPECT() *MockEventHandlerMockRecorder {
	return m.recorder
}

// GetEvents mocks base method.
func (m *MockEventHandler) GetEvents(ctx context.Context, namespaceFunc events.NamespaceFunc) ([]*models.Event, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetEvents", ctx, namespaceFunc)
	ret0, _ := ret[0].([]*models.Event)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetEvents indicates an expected call of GetEvents.
func (mr *MockEventHandlerMockRecorder) GetEvents(ctx, namespaceFunc interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetEvents", reflect.TypeOf((*MockEventHandler)(nil).GetEvents), ctx, namespaceFunc)
}
