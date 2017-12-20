// Code generated by protoc-gen-go. DO NOT EDIT.
// source: findnode.proto

/*
Package pb is a generated protocol buffer package.

It is generated from these files:
	findnode.proto
	message.proto
	nodeinfo.proto

It has these top-level messages:
	FindNodeReq
	FindNodeResp
	CommonMessageData
	HandshakeData
	ProtocolMessage
	Metadata
	PingReqData
	PingRespData
	NodeInfo
*/
package pb

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// example protocol
type FindNodeReq struct {
	Metadata   *Metadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	NodeId     []byte    `protobuf:"bytes,2,opt,name=nodeId,proto3" json:"nodeId,omitempty"`
	MaxResults int32     `protobuf:"varint,3,opt,name=maxResults" json:"maxResults,omitempty"`
}

func (m *FindNodeReq) Reset()                    { *m = FindNodeReq{} }
func (m *FindNodeReq) String() string            { return proto.CompactTextString(m) }
func (*FindNodeReq) ProtoMessage()               {}
func (*FindNodeReq) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *FindNodeReq) GetMetadata() *Metadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *FindNodeReq) GetNodeId() []byte {
	if m != nil {
		return m.NodeId
	}
	return nil
}

func (m *FindNodeReq) GetMaxResults() int32 {
	if m != nil {
		return m.MaxResults
	}
	return 0
}

type FindNodeResp struct {
	Metadata  *Metadata   `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	NodeInfos []*NodeInfo `protobuf:"bytes,2,rep,name=nodeInfos" json:"nodeInfos,omitempty"`
}

func (m *FindNodeResp) Reset()                    { *m = FindNodeResp{} }
func (m *FindNodeResp) String() string            { return proto.CompactTextString(m) }
func (*FindNodeResp) ProtoMessage()               {}
func (*FindNodeResp) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *FindNodeResp) GetMetadata() *Metadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *FindNodeResp) GetNodeInfos() []*NodeInfo {
	if m != nil {
		return m.NodeInfos
	}
	return nil
}

func init() {
	proto.RegisterType((*FindNodeReq)(nil), "pb.FindNodeReq")
	proto.RegisterType((*FindNodeResp)(nil), "pb.FindNodeResp")
}

func init() { proto.RegisterFile("findnode.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 191 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x4b, 0xcb, 0xcc, 0x4b,
	0xc9, 0xcb, 0x4f, 0x49, 0xd5, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x2a, 0x48, 0x92, 0xe2,
	0xcd, 0x4d, 0x2d, 0x2e, 0x4e, 0x4c, 0x87, 0x0a, 0x49, 0xf1, 0x81, 0xa4, 0x33, 0xf3, 0xd2, 0xf2,
	0x21, 0x7c, 0xa5, 0x7c, 0x2e, 0x6e, 0xb7, 0xcc, 0xbc, 0x14, 0xbf, 0xfc, 0x94, 0xd4, 0xa0, 0xd4,
	0x42, 0x21, 0x0d, 0x2e, 0x8e, 0xdc, 0xd4, 0x92, 0xc4, 0x94, 0xc4, 0x92, 0x44, 0x09, 0x46, 0x05,
	0x46, 0x0d, 0x6e, 0x23, 0x1e, 0xbd, 0x82, 0x24, 0x3d, 0x5f, 0xa8, 0x58, 0x10, 0x5c, 0x56, 0x48,
	0x8c, 0x8b, 0x0d, 0x64, 0x94, 0x67, 0x8a, 0x04, 0x93, 0x02, 0xa3, 0x06, 0x4f, 0x10, 0x94, 0x27,
	0x24, 0xc7, 0xc5, 0x95, 0x9b, 0x58, 0x11, 0x94, 0x5a, 0x5c, 0x9a, 0x53, 0x52, 0x2c, 0xc1, 0xac,
	0xc0, 0xa8, 0xc1, 0x1a, 0x84, 0x24, 0xa2, 0x94, 0xc2, 0xc5, 0x83, 0xb0, 0xb0, 0xb8, 0x80, 0x04,
	0x1b, 0xb5, 0xb8, 0x38, 0xc1, 0x76, 0xe4, 0xa5, 0xe5, 0x17, 0x4b, 0x30, 0x29, 0x30, 0xc3, 0x94,
	0xfa, 0x41, 0x05, 0x83, 0x10, 0xd2, 0x4e, 0x2c, 0x51, 0x4c, 0x05, 0x49, 0x49, 0x6c, 0x60, 0x3f,
	0x1a, 0x03, 0x02, 0x00, 0x00, 0xff, 0xff, 0x1c, 0xfe, 0xba, 0x1b, 0x18, 0x01, 0x00, 0x00,
}
