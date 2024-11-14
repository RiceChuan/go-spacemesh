// Code generated by MockGen. DO NOT EDIT.
// Source: ./sync.go
//
// Generated by this command:
//
//	mockgen -typed -package=mocks -destination=./mocks/sync.go -source=./sync.go
//

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockSyncStateProvider is a mock of SyncStateProvider interface.
type MockSyncStateProvider struct {
	ctrl     *gomock.Controller
	recorder *MockSyncStateProviderMockRecorder
	isgomock struct{}
}

// MockSyncStateProviderMockRecorder is the mock recorder for MockSyncStateProvider.
type MockSyncStateProviderMockRecorder struct {
	mock *MockSyncStateProvider
}

// NewMockSyncStateProvider creates a new mock instance.
func NewMockSyncStateProvider(ctrl *gomock.Controller) *MockSyncStateProvider {
	mock := &MockSyncStateProvider{ctrl: ctrl}
	mock.recorder = &MockSyncStateProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSyncStateProvider) EXPECT() *MockSyncStateProviderMockRecorder {
	return m.recorder
}

// IsSynced mocks base method.
func (m *MockSyncStateProvider) IsSynced(arg0 context.Context) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsSynced", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsSynced indicates an expected call of IsSynced.
func (mr *MockSyncStateProviderMockRecorder) IsSynced(arg0 any) *MockSyncStateProviderIsSyncedCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsSynced", reflect.TypeOf((*MockSyncStateProvider)(nil).IsSynced), arg0)
	return &MockSyncStateProviderIsSyncedCall{Call: call}
}

// MockSyncStateProviderIsSyncedCall wrap *gomock.Call
type MockSyncStateProviderIsSyncedCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockSyncStateProviderIsSyncedCall) Return(arg0 bool) *MockSyncStateProviderIsSyncedCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockSyncStateProviderIsSyncedCall) Do(f func(context.Context) bool) *MockSyncStateProviderIsSyncedCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockSyncStateProviderIsSyncedCall) DoAndReturn(f func(context.Context) bool) *MockSyncStateProviderIsSyncedCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
