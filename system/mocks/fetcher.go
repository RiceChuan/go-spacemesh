// Code generated by MockGen. DO NOT EDIT.
// Source: ./fetcher.go
//
// Generated by this command:
//
//	mockgen -typed -package=mocks -destination=./mocks/fetcher.go -source=./fetcher.go
//

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	types "github.com/spacemeshos/go-spacemesh/common/types"
	p2p "github.com/spacemeshos/go-spacemesh/p2p"
	system "github.com/spacemeshos/go-spacemesh/system"
	gomock "go.uber.org/mock/gomock"
)

// MockFetcher is a mock of Fetcher interface.
type MockFetcher struct {
	ctrl     *gomock.Controller
	recorder *MockFetcherMockRecorder
	isgomock struct{}
}

// MockFetcherMockRecorder is the mock recorder for MockFetcher.
type MockFetcherMockRecorder struct {
	mock *MockFetcher
}

// NewMockFetcher creates a new mock instance.
func NewMockFetcher(ctrl *gomock.Controller) *MockFetcher {
	mock := &MockFetcher{ctrl: ctrl}
	mock.recorder = &MockFetcherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFetcher) EXPECT() *MockFetcherMockRecorder {
	return m.recorder
}

// GetActiveSet mocks base method.
func (m *MockFetcher) GetActiveSet(arg0 context.Context, arg1 types.Hash32) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetActiveSet", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetActiveSet indicates an expected call of GetActiveSet.
func (mr *MockFetcherMockRecorder) GetActiveSet(arg0, arg1 any) *MockFetcherGetActiveSetCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetActiveSet", reflect.TypeOf((*MockFetcher)(nil).GetActiveSet), arg0, arg1)
	return &MockFetcherGetActiveSetCall{Call: call}
}

// MockFetcherGetActiveSetCall wrap *gomock.Call
type MockFetcherGetActiveSetCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockFetcherGetActiveSetCall) Return(arg0 error) *MockFetcherGetActiveSetCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockFetcherGetActiveSetCall) Do(f func(context.Context, types.Hash32) error) *MockFetcherGetActiveSetCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockFetcherGetActiveSetCall) DoAndReturn(f func(context.Context, types.Hash32) error) *MockFetcherGetActiveSetCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetAtxs mocks base method.
func (m *MockFetcher) GetAtxs(arg0 context.Context, arg1 []types.ATXID, arg2 ...system.GetAtxOpt) error {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetAtxs", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetAtxs indicates an expected call of GetAtxs.
func (mr *MockFetcherMockRecorder) GetAtxs(arg0, arg1 any, arg2 ...any) *MockFetcherGetAtxsCall {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAtxs", reflect.TypeOf((*MockFetcher)(nil).GetAtxs), varargs...)
	return &MockFetcherGetAtxsCall{Call: call}
}

// MockFetcherGetAtxsCall wrap *gomock.Call
type MockFetcherGetAtxsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockFetcherGetAtxsCall) Return(arg0 error) *MockFetcherGetAtxsCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockFetcherGetAtxsCall) Do(f func(context.Context, []types.ATXID, ...system.GetAtxOpt) error) *MockFetcherGetAtxsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockFetcherGetAtxsCall) DoAndReturn(f func(context.Context, []types.ATXID, ...system.GetAtxOpt) error) *MockFetcherGetAtxsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetBallots mocks base method.
func (m *MockFetcher) GetBallots(arg0 context.Context, arg1 []types.BallotID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBallots", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetBallots indicates an expected call of GetBallots.
func (mr *MockFetcherMockRecorder) GetBallots(arg0, arg1 any) *MockFetcherGetBallotsCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBallots", reflect.TypeOf((*MockFetcher)(nil).GetBallots), arg0, arg1)
	return &MockFetcherGetBallotsCall{Call: call}
}

// MockFetcherGetBallotsCall wrap *gomock.Call
type MockFetcherGetBallotsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockFetcherGetBallotsCall) Return(arg0 error) *MockFetcherGetBallotsCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockFetcherGetBallotsCall) Do(f func(context.Context, []types.BallotID) error) *MockFetcherGetBallotsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockFetcherGetBallotsCall) DoAndReturn(f func(context.Context, []types.BallotID) error) *MockFetcherGetBallotsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetBlockTxs mocks base method.
func (m *MockFetcher) GetBlockTxs(arg0 context.Context, arg1 []types.TransactionID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBlockTxs", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetBlockTxs indicates an expected call of GetBlockTxs.
func (mr *MockFetcherMockRecorder) GetBlockTxs(arg0, arg1 any) *MockFetcherGetBlockTxsCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockTxs", reflect.TypeOf((*MockFetcher)(nil).GetBlockTxs), arg0, arg1)
	return &MockFetcherGetBlockTxsCall{Call: call}
}

// MockFetcherGetBlockTxsCall wrap *gomock.Call
type MockFetcherGetBlockTxsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockFetcherGetBlockTxsCall) Return(arg0 error) *MockFetcherGetBlockTxsCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockFetcherGetBlockTxsCall) Do(f func(context.Context, []types.TransactionID) error) *MockFetcherGetBlockTxsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockFetcherGetBlockTxsCall) DoAndReturn(f func(context.Context, []types.TransactionID) error) *MockFetcherGetBlockTxsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetBlocks mocks base method.
func (m *MockFetcher) GetBlocks(arg0 context.Context, arg1 []types.BlockID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBlocks", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetBlocks indicates an expected call of GetBlocks.
func (mr *MockFetcherMockRecorder) GetBlocks(arg0, arg1 any) *MockFetcherGetBlocksCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlocks", reflect.TypeOf((*MockFetcher)(nil).GetBlocks), arg0, arg1)
	return &MockFetcherGetBlocksCall{Call: call}
}

// MockFetcherGetBlocksCall wrap *gomock.Call
type MockFetcherGetBlocksCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockFetcherGetBlocksCall) Return(arg0 error) *MockFetcherGetBlocksCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockFetcherGetBlocksCall) Do(f func(context.Context, []types.BlockID) error) *MockFetcherGetBlocksCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockFetcherGetBlocksCall) DoAndReturn(f func(context.Context, []types.BlockID) error) *MockFetcherGetBlocksCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetPoetProof mocks base method.
func (m *MockFetcher) GetPoetProof(arg0 context.Context, arg1 types.Hash32) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPoetProof", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetPoetProof indicates an expected call of GetPoetProof.
func (mr *MockFetcherMockRecorder) GetPoetProof(arg0, arg1 any) *MockFetcherGetPoetProofCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPoetProof", reflect.TypeOf((*MockFetcher)(nil).GetPoetProof), arg0, arg1)
	return &MockFetcherGetPoetProofCall{Call: call}
}

// MockFetcherGetPoetProofCall wrap *gomock.Call
type MockFetcherGetPoetProofCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockFetcherGetPoetProofCall) Return(arg0 error) *MockFetcherGetPoetProofCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockFetcherGetPoetProofCall) Do(f func(context.Context, types.Hash32) error) *MockFetcherGetPoetProofCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockFetcherGetPoetProofCall) DoAndReturn(f func(context.Context, types.Hash32) error) *MockFetcherGetPoetProofCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetProposalTxs mocks base method.
func (m *MockFetcher) GetProposalTxs(arg0 context.Context, arg1 []types.TransactionID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetProposalTxs", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetProposalTxs indicates an expected call of GetProposalTxs.
func (mr *MockFetcherMockRecorder) GetProposalTxs(arg0, arg1 any) *MockFetcherGetProposalTxsCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetProposalTxs", reflect.TypeOf((*MockFetcher)(nil).GetProposalTxs), arg0, arg1)
	return &MockFetcherGetProposalTxsCall{Call: call}
}

// MockFetcherGetProposalTxsCall wrap *gomock.Call
type MockFetcherGetProposalTxsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockFetcherGetProposalTxsCall) Return(arg0 error) *MockFetcherGetProposalTxsCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockFetcherGetProposalTxsCall) Do(f func(context.Context, []types.TransactionID) error) *MockFetcherGetProposalTxsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockFetcherGetProposalTxsCall) DoAndReturn(f func(context.Context, []types.TransactionID) error) *MockFetcherGetProposalTxsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetProposals mocks base method.
func (m *MockFetcher) GetProposals(arg0 context.Context, arg1 []types.ProposalID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetProposals", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetProposals indicates an expected call of GetProposals.
func (mr *MockFetcherMockRecorder) GetProposals(arg0, arg1 any) *MockFetcherGetProposalsCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetProposals", reflect.TypeOf((*MockFetcher)(nil).GetProposals), arg0, arg1)
	return &MockFetcherGetProposalsCall{Call: call}
}

// MockFetcherGetProposalsCall wrap *gomock.Call
type MockFetcherGetProposalsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockFetcherGetProposalsCall) Return(arg0 error) *MockFetcherGetProposalsCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockFetcherGetProposalsCall) Do(f func(context.Context, []types.ProposalID) error) *MockFetcherGetProposalsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockFetcherGetProposalsCall) DoAndReturn(f func(context.Context, []types.ProposalID) error) *MockFetcherGetProposalsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// RegisterPeerHashes mocks base method.
func (m *MockFetcher) RegisterPeerHashes(peer p2p.Peer, hashes []types.Hash32) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RegisterPeerHashes", peer, hashes)
}

// RegisterPeerHashes indicates an expected call of RegisterPeerHashes.
func (mr *MockFetcherMockRecorder) RegisterPeerHashes(peer, hashes any) *MockFetcherRegisterPeerHashesCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterPeerHashes", reflect.TypeOf((*MockFetcher)(nil).RegisterPeerHashes), peer, hashes)
	return &MockFetcherRegisterPeerHashesCall{Call: call}
}

// MockFetcherRegisterPeerHashesCall wrap *gomock.Call
type MockFetcherRegisterPeerHashesCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockFetcherRegisterPeerHashesCall) Return() *MockFetcherRegisterPeerHashesCall {
	c.Call = c.Call.Return()
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockFetcherRegisterPeerHashesCall) Do(f func(p2p.Peer, []types.Hash32)) *MockFetcherRegisterPeerHashesCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockFetcherRegisterPeerHashesCall) DoAndReturn(f func(p2p.Peer, []types.Hash32)) *MockFetcherRegisterPeerHashesCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockBlockFetcher is a mock of BlockFetcher interface.
type MockBlockFetcher struct {
	ctrl     *gomock.Controller
	recorder *MockBlockFetcherMockRecorder
	isgomock struct{}
}

// MockBlockFetcherMockRecorder is the mock recorder for MockBlockFetcher.
type MockBlockFetcherMockRecorder struct {
	mock *MockBlockFetcher
}

// NewMockBlockFetcher creates a new mock instance.
func NewMockBlockFetcher(ctrl *gomock.Controller) *MockBlockFetcher {
	mock := &MockBlockFetcher{ctrl: ctrl}
	mock.recorder = &MockBlockFetcherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBlockFetcher) EXPECT() *MockBlockFetcherMockRecorder {
	return m.recorder
}

// GetBlocks mocks base method.
func (m *MockBlockFetcher) GetBlocks(arg0 context.Context, arg1 []types.BlockID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBlocks", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetBlocks indicates an expected call of GetBlocks.
func (mr *MockBlockFetcherMockRecorder) GetBlocks(arg0, arg1 any) *MockBlockFetcherGetBlocksCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlocks", reflect.TypeOf((*MockBlockFetcher)(nil).GetBlocks), arg0, arg1)
	return &MockBlockFetcherGetBlocksCall{Call: call}
}

// MockBlockFetcherGetBlocksCall wrap *gomock.Call
type MockBlockFetcherGetBlocksCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockBlockFetcherGetBlocksCall) Return(arg0 error) *MockBlockFetcherGetBlocksCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockBlockFetcherGetBlocksCall) Do(f func(context.Context, []types.BlockID) error) *MockBlockFetcherGetBlocksCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockBlockFetcherGetBlocksCall) DoAndReturn(f func(context.Context, []types.BlockID) error) *MockBlockFetcherGetBlocksCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockAtxFetcher is a mock of AtxFetcher interface.
type MockAtxFetcher struct {
	ctrl     *gomock.Controller
	recorder *MockAtxFetcherMockRecorder
	isgomock struct{}
}

// MockAtxFetcherMockRecorder is the mock recorder for MockAtxFetcher.
type MockAtxFetcherMockRecorder struct {
	mock *MockAtxFetcher
}

// NewMockAtxFetcher creates a new mock instance.
func NewMockAtxFetcher(ctrl *gomock.Controller) *MockAtxFetcher {
	mock := &MockAtxFetcher{ctrl: ctrl}
	mock.recorder = &MockAtxFetcherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAtxFetcher) EXPECT() *MockAtxFetcherMockRecorder {
	return m.recorder
}

// GetAtxs mocks base method.
func (m *MockAtxFetcher) GetAtxs(arg0 context.Context, arg1 []types.ATXID, arg2 ...system.GetAtxOpt) error {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetAtxs", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetAtxs indicates an expected call of GetAtxs.
func (mr *MockAtxFetcherMockRecorder) GetAtxs(arg0, arg1 any, arg2 ...any) *MockAtxFetcherGetAtxsCall {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAtxs", reflect.TypeOf((*MockAtxFetcher)(nil).GetAtxs), varargs...)
	return &MockAtxFetcherGetAtxsCall{Call: call}
}

// MockAtxFetcherGetAtxsCall wrap *gomock.Call
type MockAtxFetcherGetAtxsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockAtxFetcherGetAtxsCall) Return(arg0 error) *MockAtxFetcherGetAtxsCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockAtxFetcherGetAtxsCall) Do(f func(context.Context, []types.ATXID, ...system.GetAtxOpt) error) *MockAtxFetcherGetAtxsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockAtxFetcherGetAtxsCall) DoAndReturn(f func(context.Context, []types.ATXID, ...system.GetAtxOpt) error) *MockAtxFetcherGetAtxsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockTxFetcher is a mock of TxFetcher interface.
type MockTxFetcher struct {
	ctrl     *gomock.Controller
	recorder *MockTxFetcherMockRecorder
	isgomock struct{}
}

// MockTxFetcherMockRecorder is the mock recorder for MockTxFetcher.
type MockTxFetcherMockRecorder struct {
	mock *MockTxFetcher
}

// NewMockTxFetcher creates a new mock instance.
func NewMockTxFetcher(ctrl *gomock.Controller) *MockTxFetcher {
	mock := &MockTxFetcher{ctrl: ctrl}
	mock.recorder = &MockTxFetcherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTxFetcher) EXPECT() *MockTxFetcherMockRecorder {
	return m.recorder
}

// GetBlockTxs mocks base method.
func (m *MockTxFetcher) GetBlockTxs(arg0 context.Context, arg1 []types.TransactionID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBlockTxs", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetBlockTxs indicates an expected call of GetBlockTxs.
func (mr *MockTxFetcherMockRecorder) GetBlockTxs(arg0, arg1 any) *MockTxFetcherGetBlockTxsCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockTxs", reflect.TypeOf((*MockTxFetcher)(nil).GetBlockTxs), arg0, arg1)
	return &MockTxFetcherGetBlockTxsCall{Call: call}
}

// MockTxFetcherGetBlockTxsCall wrap *gomock.Call
type MockTxFetcherGetBlockTxsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockTxFetcherGetBlockTxsCall) Return(arg0 error) *MockTxFetcherGetBlockTxsCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockTxFetcherGetBlockTxsCall) Do(f func(context.Context, []types.TransactionID) error) *MockTxFetcherGetBlockTxsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockTxFetcherGetBlockTxsCall) DoAndReturn(f func(context.Context, []types.TransactionID) error) *MockTxFetcherGetBlockTxsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// GetProposalTxs mocks base method.
func (m *MockTxFetcher) GetProposalTxs(arg0 context.Context, arg1 []types.TransactionID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetProposalTxs", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetProposalTxs indicates an expected call of GetProposalTxs.
func (mr *MockTxFetcherMockRecorder) GetProposalTxs(arg0, arg1 any) *MockTxFetcherGetProposalTxsCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetProposalTxs", reflect.TypeOf((*MockTxFetcher)(nil).GetProposalTxs), arg0, arg1)
	return &MockTxFetcherGetProposalTxsCall{Call: call}
}

// MockTxFetcherGetProposalTxsCall wrap *gomock.Call
type MockTxFetcherGetProposalTxsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockTxFetcherGetProposalTxsCall) Return(arg0 error) *MockTxFetcherGetProposalTxsCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockTxFetcherGetProposalTxsCall) Do(f func(context.Context, []types.TransactionID) error) *MockTxFetcherGetProposalTxsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockTxFetcherGetProposalTxsCall) DoAndReturn(f func(context.Context, []types.TransactionID) error) *MockTxFetcherGetProposalTxsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockPoetProofFetcher is a mock of PoetProofFetcher interface.
type MockPoetProofFetcher struct {
	ctrl     *gomock.Controller
	recorder *MockPoetProofFetcherMockRecorder
	isgomock struct{}
}

// MockPoetProofFetcherMockRecorder is the mock recorder for MockPoetProofFetcher.
type MockPoetProofFetcherMockRecorder struct {
	mock *MockPoetProofFetcher
}

// NewMockPoetProofFetcher creates a new mock instance.
func NewMockPoetProofFetcher(ctrl *gomock.Controller) *MockPoetProofFetcher {
	mock := &MockPoetProofFetcher{ctrl: ctrl}
	mock.recorder = &MockPoetProofFetcherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPoetProofFetcher) EXPECT() *MockPoetProofFetcherMockRecorder {
	return m.recorder
}

// GetPoetProof mocks base method.
func (m *MockPoetProofFetcher) GetPoetProof(arg0 context.Context, arg1 types.Hash32) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPoetProof", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetPoetProof indicates an expected call of GetPoetProof.
func (mr *MockPoetProofFetcherMockRecorder) GetPoetProof(arg0, arg1 any) *MockPoetProofFetcherGetPoetProofCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPoetProof", reflect.TypeOf((*MockPoetProofFetcher)(nil).GetPoetProof), arg0, arg1)
	return &MockPoetProofFetcherGetPoetProofCall{Call: call}
}

// MockPoetProofFetcherGetPoetProofCall wrap *gomock.Call
type MockPoetProofFetcherGetPoetProofCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockPoetProofFetcherGetPoetProofCall) Return(arg0 error) *MockPoetProofFetcherGetPoetProofCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockPoetProofFetcherGetPoetProofCall) Do(f func(context.Context, types.Hash32) error) *MockPoetProofFetcherGetPoetProofCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockPoetProofFetcherGetPoetProofCall) DoAndReturn(f func(context.Context, types.Hash32) error) *MockPoetProofFetcherGetPoetProofCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockBallotFetcher is a mock of BallotFetcher interface.
type MockBallotFetcher struct {
	ctrl     *gomock.Controller
	recorder *MockBallotFetcherMockRecorder
	isgomock struct{}
}

// MockBallotFetcherMockRecorder is the mock recorder for MockBallotFetcher.
type MockBallotFetcherMockRecorder struct {
	mock *MockBallotFetcher
}

// NewMockBallotFetcher creates a new mock instance.
func NewMockBallotFetcher(ctrl *gomock.Controller) *MockBallotFetcher {
	mock := &MockBallotFetcher{ctrl: ctrl}
	mock.recorder = &MockBallotFetcherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBallotFetcher) EXPECT() *MockBallotFetcherMockRecorder {
	return m.recorder
}

// GetBallots mocks base method.
func (m *MockBallotFetcher) GetBallots(arg0 context.Context, arg1 []types.BallotID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBallots", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetBallots indicates an expected call of GetBallots.
func (mr *MockBallotFetcherMockRecorder) GetBallots(arg0, arg1 any) *MockBallotFetcherGetBallotsCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBallots", reflect.TypeOf((*MockBallotFetcher)(nil).GetBallots), arg0, arg1)
	return &MockBallotFetcherGetBallotsCall{Call: call}
}

// MockBallotFetcherGetBallotsCall wrap *gomock.Call
type MockBallotFetcherGetBallotsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockBallotFetcherGetBallotsCall) Return(arg0 error) *MockBallotFetcherGetBallotsCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockBallotFetcherGetBallotsCall) Do(f func(context.Context, []types.BallotID) error) *MockBallotFetcherGetBallotsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockBallotFetcherGetBallotsCall) DoAndReturn(f func(context.Context, []types.BallotID) error) *MockBallotFetcherGetBallotsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockProposalFetcher is a mock of ProposalFetcher interface.
type MockProposalFetcher struct {
	ctrl     *gomock.Controller
	recorder *MockProposalFetcherMockRecorder
	isgomock struct{}
}

// MockProposalFetcherMockRecorder is the mock recorder for MockProposalFetcher.
type MockProposalFetcherMockRecorder struct {
	mock *MockProposalFetcher
}

// NewMockProposalFetcher creates a new mock instance.
func NewMockProposalFetcher(ctrl *gomock.Controller) *MockProposalFetcher {
	mock := &MockProposalFetcher{ctrl: ctrl}
	mock.recorder = &MockProposalFetcherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockProposalFetcher) EXPECT() *MockProposalFetcherMockRecorder {
	return m.recorder
}

// GetProposals mocks base method.
func (m *MockProposalFetcher) GetProposals(arg0 context.Context, arg1 []types.ProposalID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetProposals", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetProposals indicates an expected call of GetProposals.
func (mr *MockProposalFetcherMockRecorder) GetProposals(arg0, arg1 any) *MockProposalFetcherGetProposalsCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetProposals", reflect.TypeOf((*MockProposalFetcher)(nil).GetProposals), arg0, arg1)
	return &MockProposalFetcherGetProposalsCall{Call: call}
}

// MockProposalFetcherGetProposalsCall wrap *gomock.Call
type MockProposalFetcherGetProposalsCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockProposalFetcherGetProposalsCall) Return(arg0 error) *MockProposalFetcherGetProposalsCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockProposalFetcherGetProposalsCall) Do(f func(context.Context, []types.ProposalID) error) *MockProposalFetcherGetProposalsCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockProposalFetcherGetProposalsCall) DoAndReturn(f func(context.Context, []types.ProposalID) error) *MockProposalFetcherGetProposalsCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockActiveSetFetcher is a mock of ActiveSetFetcher interface.
type MockActiveSetFetcher struct {
	ctrl     *gomock.Controller
	recorder *MockActiveSetFetcherMockRecorder
	isgomock struct{}
}

// MockActiveSetFetcherMockRecorder is the mock recorder for MockActiveSetFetcher.
type MockActiveSetFetcherMockRecorder struct {
	mock *MockActiveSetFetcher
}

// NewMockActiveSetFetcher creates a new mock instance.
func NewMockActiveSetFetcher(ctrl *gomock.Controller) *MockActiveSetFetcher {
	mock := &MockActiveSetFetcher{ctrl: ctrl}
	mock.recorder = &MockActiveSetFetcherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockActiveSetFetcher) EXPECT() *MockActiveSetFetcherMockRecorder {
	return m.recorder
}

// GetActiveSet mocks base method.
func (m *MockActiveSetFetcher) GetActiveSet(arg0 context.Context, arg1 types.Hash32) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetActiveSet", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetActiveSet indicates an expected call of GetActiveSet.
func (mr *MockActiveSetFetcherMockRecorder) GetActiveSet(arg0, arg1 any) *MockActiveSetFetcherGetActiveSetCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetActiveSet", reflect.TypeOf((*MockActiveSetFetcher)(nil).GetActiveSet), arg0, arg1)
	return &MockActiveSetFetcherGetActiveSetCall{Call: call}
}

// MockActiveSetFetcherGetActiveSetCall wrap *gomock.Call
type MockActiveSetFetcherGetActiveSetCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockActiveSetFetcherGetActiveSetCall) Return(arg0 error) *MockActiveSetFetcherGetActiveSetCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockActiveSetFetcherGetActiveSetCall) Do(f func(context.Context, types.Hash32) error) *MockActiveSetFetcherGetActiveSetCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockActiveSetFetcherGetActiveSetCall) DoAndReturn(f func(context.Context, types.Hash32) error) *MockActiveSetFetcherGetActiveSetCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// MockPeerTracker is a mock of PeerTracker interface.
type MockPeerTracker struct {
	ctrl     *gomock.Controller
	recorder *MockPeerTrackerMockRecorder
	isgomock struct{}
}

// MockPeerTrackerMockRecorder is the mock recorder for MockPeerTracker.
type MockPeerTrackerMockRecorder struct {
	mock *MockPeerTracker
}

// NewMockPeerTracker creates a new mock instance.
func NewMockPeerTracker(ctrl *gomock.Controller) *MockPeerTracker {
	mock := &MockPeerTracker{ctrl: ctrl}
	mock.recorder = &MockPeerTrackerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPeerTracker) EXPECT() *MockPeerTrackerMockRecorder {
	return m.recorder
}

// RegisterPeerHashes mocks base method.
func (m *MockPeerTracker) RegisterPeerHashes(peer p2p.Peer, hashes []types.Hash32) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RegisterPeerHashes", peer, hashes)
}

// RegisterPeerHashes indicates an expected call of RegisterPeerHashes.
func (mr *MockPeerTrackerMockRecorder) RegisterPeerHashes(peer, hashes any) *MockPeerTrackerRegisterPeerHashesCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterPeerHashes", reflect.TypeOf((*MockPeerTracker)(nil).RegisterPeerHashes), peer, hashes)
	return &MockPeerTrackerRegisterPeerHashesCall{Call: call}
}

// MockPeerTrackerRegisterPeerHashesCall wrap *gomock.Call
type MockPeerTrackerRegisterPeerHashesCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockPeerTrackerRegisterPeerHashesCall) Return() *MockPeerTrackerRegisterPeerHashesCall {
	c.Call = c.Call.Return()
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockPeerTrackerRegisterPeerHashesCall) Do(f func(p2p.Peer, []types.Hash32)) *MockPeerTrackerRegisterPeerHashesCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockPeerTrackerRegisterPeerHashesCall) DoAndReturn(f func(p2p.Peer, []types.Hash32)) *MockPeerTrackerRegisterPeerHashesCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
