// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/utils/tlsvalidator/interface.go

// Package mock is a generated GoMock package.
package mock

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockTLSSecretValidator is a mock of TLSSecretValidator interface.
type MockTLSSecretValidator struct {
	ctrl     *gomock.Controller
	recorder *MockTLSSecretValidatorMockRecorder
}

// MockTLSSecretValidatorMockRecorder is the mock recorder for MockTLSSecretValidator.
type MockTLSSecretValidatorMockRecorder struct {
	mock *MockTLSSecretValidator
}

// NewMockTLSSecretValidator creates a new mock instance.
func NewMockTLSSecretValidator(ctrl *gomock.Controller) *MockTLSSecretValidator {
	mock := &MockTLSSecretValidator{ctrl: ctrl}
	mock.recorder = &MockTLSSecretValidatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTLSSecretValidator) EXPECT() *MockTLSSecretValidatorMockRecorder {
	return m.recorder
}

// ValidateTLSCertificate mocks base method.
func (m *MockTLSSecretValidator) ValidateTLSCertificate(certBytes, keyBytes []byte, dnsName string) (bool, []string) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidateTLSCertificate", certBytes, keyBytes, dnsName)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].([]string)
	return ret0, ret1
}

// ValidateTLSCertificate indicates an expected call of ValidateTLSCertificate.
func (mr *MockTLSSecretValidatorMockRecorder) ValidateTLSCertificate(certBytes, keyBytes, dnsName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidateTLSCertificate", reflect.TypeOf((*MockTLSSecretValidator)(nil).ValidateTLSCertificate), certBytes, keyBytes, dnsName)
}

// ValidateTLSKey mocks base method.
func (m *MockTLSSecretValidator) ValidateTLSKey(keyBytes []byte) (bool, []string) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidateTLSKey", keyBytes)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].([]string)
	return ret0, ret1
}

// ValidateTLSKey indicates an expected call of ValidateTLSKey.
func (mr *MockTLSSecretValidatorMockRecorder) ValidateTLSKey(keyBytes interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidateTLSKey", reflect.TypeOf((*MockTLSSecretValidator)(nil).ValidateTLSKey), keyBytes)
}
