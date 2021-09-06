// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/environmentvariables/env_vars_handler_factory.go

// Package environmentvariables is a generated GoMock package.
package environmentvariables

import (
	reflect "reflect"

	models "github.com/equinor/radix-api/models"
	gomock "github.com/golang/mock/gomock"
)

// MockenvVarsHandlerFactory is a mock of envVarsHandlerFactory interface.
type MockenvVarsHandlerFactory struct {
	ctrl     *gomock.Controller
	recorder *MockenvVarsHandlerFactoryMockRecorder
}

// MockenvVarsHandlerFactoryMockRecorder is the mock recorder for MockenvVarsHandlerFactory.
type MockenvVarsHandlerFactoryMockRecorder struct {
	mock *MockenvVarsHandlerFactory
}

// NewMockenvVarsHandlerFactory creates a new mock instance.
func NewMockenvVarsHandlerFactory(ctrl *gomock.Controller) *MockenvVarsHandlerFactory {
	mock := &MockenvVarsHandlerFactory{ctrl: ctrl}
	mock.recorder = &MockenvVarsHandlerFactoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockenvVarsHandlerFactory) EXPECT() *MockenvVarsHandlerFactoryMockRecorder {
	return m.recorder
}

// createHandler mocks base method.
func (m *MockenvVarsHandlerFactory) createHandler(arg0 models.Accounts) EnvVarsHandler {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "createHandler", arg0)
	ret0, _ := ret[0].(EnvVarsHandler)
	return ret0
}

// createHandler indicates an expected call of createHandler.
func (mr *MockenvVarsHandlerFactoryMockRecorder) createHandler(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "createHandler", reflect.TypeOf((*MockenvVarsHandlerFactory)(nil).createHandler), arg0)
}
