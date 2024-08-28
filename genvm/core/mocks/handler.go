// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/spacemeshos/go-spacemesh/genvm/core (interfaces: Handler)
//
// Generated by this command:
//
//	mockgen -typed -package=mocks -destination=./mocks/handler.go github.com/spacemeshos/go-spacemesh/genvm/core Handler
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	scale "github.com/spacemeshos/go-scale"
	core "github.com/spacemeshos/go-spacemesh/genvm/core"
	gomock "go.uber.org/mock/gomock"
)

// MockHandler is a mock of Handler interface.
type MockHandler struct {
	ctrl     *gomock.Controller
	recorder *MockHandlerMockRecorder
}

// MockHandlerMockRecorder is the mock recorder for MockHandler.
type MockHandlerMockRecorder struct {
	mock *MockHandler
}

// NewMockHandler creates a new mock instance.
func NewMockHandler(ctrl *gomock.Controller) *MockHandler {
	mock := &MockHandler{ctrl: ctrl}
	mock.recorder = &MockHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHandler) EXPECT() *MockHandlerMockRecorder {
	return m.recorder
}

// Args mocks base method.
func (m *MockHandler) Args(arg0 byte) scale.Type {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Args", arg0)
	ret0, _ := ret[0].(scale.Type)
	return ret0
}

// Args indicates an expected call of Args.
func (mr *MockHandlerMockRecorder) Args(arg0 any) *MockHandlerArgsCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Args", reflect.TypeOf((*MockHandler)(nil).Args), arg0)
	return &MockHandlerArgsCall{Call: call}
}

// MockHandlerArgsCall wrap *gomock.Call
type MockHandlerArgsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockHandlerArgsCall) Return(arg0 scale.Type) *MockHandlerArgsCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockHandlerArgsCall) Do(f func(byte) scale.Type) *MockHandlerArgsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockHandlerArgsCall) DoAndReturn(f func(byte) scale.Type) *MockHandlerArgsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Exec mocks base method.
func (m *MockHandler) Exec(arg0 core.Host, arg1 byte, arg2 scale.Encodable) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Exec", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Exec indicates an expected call of Exec.
func (mr *MockHandlerMockRecorder) Exec(arg0, arg1, arg2 any) *MockHandlerExecCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Exec", reflect.TypeOf((*MockHandler)(nil).Exec), arg0, arg1, arg2)
	return &MockHandlerExecCall{Call: call}
}

// MockHandlerExecCall wrap *gomock.Call
type MockHandlerExecCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockHandlerExecCall) Return(arg0 error) *MockHandlerExecCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockHandlerExecCall) Do(f func(core.Host, byte, scale.Encodable) error) *MockHandlerExecCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockHandlerExecCall) DoAndReturn(f func(core.Host, byte, scale.Encodable) error) *MockHandlerExecCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Load mocks base method.
func (m *MockHandler) Load(arg0 []byte) (core.Template, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Load", arg0)
	ret0, _ := ret[0].(core.Template)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Load indicates an expected call of Load.
func (mr *MockHandlerMockRecorder) Load(arg0 any) *MockHandlerLoadCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Load", reflect.TypeOf((*MockHandler)(nil).Load), arg0)
	return &MockHandlerLoadCall{Call: call}
}

// MockHandlerLoadCall wrap *gomock.Call
type MockHandlerLoadCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockHandlerLoadCall) Return(arg0 core.Template, arg1 error) *MockHandlerLoadCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockHandlerLoadCall) Do(f func([]byte) (core.Template, error)) *MockHandlerLoadCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockHandlerLoadCall) DoAndReturn(f func([]byte) (core.Template, error)) *MockHandlerLoadCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// New mocks base method.
func (m *MockHandler) New(arg0 any) (core.Template, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "New", arg0)
	ret0, _ := ret[0].(core.Template)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// New indicates an expected call of New.
func (mr *MockHandlerMockRecorder) New(arg0 any) *MockHandlerNewCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "New", reflect.TypeOf((*MockHandler)(nil).New), arg0)
	return &MockHandlerNewCall{Call: call}
}

// MockHandlerNewCall wrap *gomock.Call
type MockHandlerNewCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockHandlerNewCall) Return(arg0 core.Template, arg1 error) *MockHandlerNewCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockHandlerNewCall) Do(f func(any) (core.Template, error)) *MockHandlerNewCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockHandlerNewCall) DoAndReturn(f func(any) (core.Template, error)) *MockHandlerNewCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Parse mocks base method.
func (m *MockHandler) Parse(arg0 byte, arg1 *scale.Decoder) (core.ParseOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Parse", arg0, arg1)
	ret0, _ := ret[0].(core.ParseOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Parse indicates an expected call of Parse.
func (mr *MockHandlerMockRecorder) Parse(arg0, arg1 any) *MockHandlerParseCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Parse", reflect.TypeOf((*MockHandler)(nil).Parse), arg0, arg1)
	return &MockHandlerParseCall{Call: call}
}

// MockHandlerParseCall wrap *gomock.Call
type MockHandlerParseCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockHandlerParseCall) Return(arg0 core.ParseOutput, arg1 error) *MockHandlerParseCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockHandlerParseCall) Do(f func(byte, *scale.Decoder) (core.ParseOutput, error)) *MockHandlerParseCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockHandlerParseCall) DoAndReturn(f func(byte, *scale.Decoder) (core.ParseOutput, error)) *MockHandlerParseCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
