// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/deployments/deployment_handler.go

// Package mock is a generated GoMock package.
package mock

import (
	reflect "reflect"
	time "time"

	models "github.com/equinor/radix-api/api/deployments/models"
	gomock "github.com/golang/mock/gomock"
)

// MockDeployHandler is a mock of DeployHandler interface.
type MockDeployHandler struct {
	ctrl     *gomock.Controller
	recorder *MockDeployHandlerMockRecorder
}

// MockDeployHandlerMockRecorder is the mock recorder for MockDeployHandler.
type MockDeployHandlerMockRecorder struct {
	mock *MockDeployHandler
}

// NewMockDeployHandler creates a new mock instance.
func NewMockDeployHandler(ctrl *gomock.Controller) *MockDeployHandler {
	mock := &MockDeployHandler{ctrl: ctrl}
	mock.recorder = &MockDeployHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDeployHandler) EXPECT() *MockDeployHandlerMockRecorder {
	return m.recorder
}

// GetComponentsForDeploymentName mocks base method.
func (m *MockDeployHandler) GetComponentsForDeploymentName(appName, deploymentID string) ([]*models.Component, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetComponentsForDeploymentName", appName, deploymentID)
	ret0, _ := ret[0].([]*models.Component)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetComponentsForDeploymentName indicates an expected call of GetComponentsForDeploymentName.
func (mr *MockDeployHandlerMockRecorder) GetComponentsForDeploymentName(appName, deploymentID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetComponentsForDeploymentName", reflect.TypeOf((*MockDeployHandler)(nil).GetComponentsForDeploymentName), appName, deploymentID)
}

// GetDeploymentWithName mocks base method.
func (m *MockDeployHandler) GetDeploymentWithName(appName, deploymentName string) (*models.Deployment, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDeploymentWithName", appName, deploymentName)
	ret0, _ := ret[0].(*models.Deployment)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDeploymentWithName indicates an expected call of GetDeploymentWithName.
func (mr *MockDeployHandlerMockRecorder) GetDeploymentWithName(appName, deploymentName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDeploymentWithName", reflect.TypeOf((*MockDeployHandler)(nil).GetDeploymentWithName), appName, deploymentName)
}

// GetDeploymentsForApplicationEnvironment mocks base method.
func (m *MockDeployHandler) GetDeploymentsForApplicationEnvironment(appName, environment string, latest bool) ([]*models.DeploymentSummary, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDeploymentsForApplicationEnvironment", appName, environment, latest)
	ret0, _ := ret[0].([]*models.DeploymentSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDeploymentsForApplicationEnvironment indicates an expected call of GetDeploymentsForApplicationEnvironment.
func (mr *MockDeployHandlerMockRecorder) GetDeploymentsForApplicationEnvironment(appName, environment, latest interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDeploymentsForApplicationEnvironment", reflect.TypeOf((*MockDeployHandler)(nil).GetDeploymentsForApplicationEnvironment), appName, environment, latest)
}

// GetDeploymentsForJob mocks base method.
func (m *MockDeployHandler) GetDeploymentsForJob(appName, jobName string) ([]*models.DeploymentSummary, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDeploymentsForJob", appName, jobName)
	ret0, _ := ret[0].([]*models.DeploymentSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDeploymentsForJob indicates an expected call of GetDeploymentsForJob.
func (mr *MockDeployHandlerMockRecorder) GetDeploymentsForJob(appName, jobName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDeploymentsForJob", reflect.TypeOf((*MockDeployHandler)(nil).GetDeploymentsForJob), appName, jobName)
}

// GetLatestDeploymentForApplicationEnvironment mocks base method.
func (m *MockDeployHandler) GetLatestDeploymentForApplicationEnvironment(appName, environment string) (*models.DeploymentSummary, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLatestDeploymentForApplicationEnvironment", appName, environment)
	ret0, _ := ret[0].(*models.DeploymentSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLatestDeploymentForApplicationEnvironment indicates an expected call of GetLatestDeploymentForApplicationEnvironment.
func (mr *MockDeployHandlerMockRecorder) GetLatestDeploymentForApplicationEnvironment(appName, environment interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLatestDeploymentForApplicationEnvironment", reflect.TypeOf((*MockDeployHandler)(nil).GetLatestDeploymentForApplicationEnvironment), appName, environment)
}

// GetLogs mocks base method.
func (m *MockDeployHandler) GetLogs(appName, podName string, sinceTime *time.Time) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLogs", appName, podName, sinceTime)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetLogs indicates an expected call of GetLogs.
func (mr *MockDeployHandlerMockRecorder) GetLogs(appName, podName, sinceTime interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLogs", reflect.TypeOf((*MockDeployHandler)(nil).GetLogs), appName, podName, sinceTime)
}