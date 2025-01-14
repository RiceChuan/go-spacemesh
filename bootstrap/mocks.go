// Code generated by MockGen. DO NOT EDIT.
// Source: ./interface.go
//
// Generated by this command:
//
//	mockgen -typed -package=bootstrap -destination=./mocks.go -source=./interface.go
//

// Package bootstrap is a generated GoMock package.
package bootstrap

import (
	reflect "reflect"

	types "github.com/spacemeshos/go-spacemesh/common/types"
	gomock "go.uber.org/mock/gomock"
)

// MocklayerClock is a mock of layerClock interface.
type MocklayerClock struct {
	ctrl     *gomock.Controller
	recorder *MocklayerClockMockRecorder
	isgomock struct{}
}

// MocklayerClockMockRecorder is the mock recorder for MocklayerClock.
type MocklayerClockMockRecorder struct {
	mock *MocklayerClock
}

// NewMocklayerClock creates a new mock instance.
func NewMocklayerClock(ctrl *gomock.Controller) *MocklayerClock {
	mock := &MocklayerClock{ctrl: ctrl}
	mock.recorder = &MocklayerClockMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MocklayerClock) EXPECT() *MocklayerClockMockRecorder {
	return m.recorder
}

// CurrentLayer mocks base method.
func (m *MocklayerClock) CurrentLayer() types.LayerID {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CurrentLayer")
	ret0, _ := ret[0].(types.LayerID)
	return ret0
}

// CurrentLayer indicates an expected call of CurrentLayer.
func (mr *MocklayerClockMockRecorder) CurrentLayer() *MocklayerClockCurrentLayerCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CurrentLayer", reflect.TypeOf((*MocklayerClock)(nil).CurrentLayer))
	return &MocklayerClockCurrentLayerCall{Call: call}
}

// MocklayerClockCurrentLayerCall wrap *gomock.Call
type MocklayerClockCurrentLayerCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MocklayerClockCurrentLayerCall) Return(arg0 types.LayerID) *MocklayerClockCurrentLayerCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MocklayerClockCurrentLayerCall) Do(f func() types.LayerID) *MocklayerClockCurrentLayerCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MocklayerClockCurrentLayerCall) DoAndReturn(f func() types.LayerID) *MocklayerClockCurrentLayerCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
