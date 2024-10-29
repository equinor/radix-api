// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/events/event_handler.go

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

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

// GetComponentEvents mocks base method.
func (m *MockEventHandler) GetComponentEvents(ctx context.Context, appName, envName, componentName string) ([]*models.Event, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetComponentEvents", ctx, appName, envName, componentName)
	ret0, _ := ret[0].([]*models.Event)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetComponentEvents indicates an expected call of GetComponentEvents.
func (mr *MockEventHandlerMockRecorder) GetComponentEvents(ctx, appName, envName, componentName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetComponentEvents", reflect.TypeOf((*MockEventHandler)(nil).GetComponentEvents), ctx, appName, envName, componentName)
}

// GetEnvironmentEvents mocks base method.
func (m *MockEventHandler) GetEnvironmentEvents(ctx context.Context, appName, envName string) ([]*models.Event, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetEnvironmentEvents", ctx, appName, envName)
	ret0, _ := ret[0].([]*models.Event)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetEnvironmentEvents indicates an expected call of GetEnvironmentEvents.
func (mr *MockEventHandlerMockRecorder) GetEnvironmentEvents(ctx, appName, envName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetEnvironmentEvents", reflect.TypeOf((*MockEventHandler)(nil).GetEnvironmentEvents), ctx, appName, envName)
}

// GetPodEvents mocks base method.
func (m *MockEventHandler) GetPodEvents(ctx context.Context, appName, envName, componentName, podName string) ([]*models.Event, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPodEvents", ctx, appName, envName, componentName, podName)
	ret0, _ := ret[0].([]*models.Event)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPodEvents indicates an expected call of GetPodEvents.
func (mr *MockEventHandlerMockRecorder) GetPodEvents(ctx, appName, envName, componentName, podName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPodEvents", reflect.TypeOf((*MockEventHandler)(nil).GetPodEvents), ctx, appName, envName, componentName, podName)
}
