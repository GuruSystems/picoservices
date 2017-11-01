// Code generated by protoc-gen-go.
// source: proto/softcat-rfc-creator.proto
// DO NOT EDIT!

/*
Package rfccreator is a generated protocol buffer package.

It is generated from these files:
	proto/softcat-rfc-creator.proto

It has these top-level messages:
	CreateRequest
	CreateResponse
	PingRequest
	PingResponse
*/
package rfccreator

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import _ "google.golang.org/genproto/googleapis/api/annotations"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

//
// import "google/protobuf/empty.proto";
// import "google/protobuf/duration.proto";
// import "examples/sub/message.proto";
// import "examples/sub2/message.proto";
// import "google/protobuf/timestamp.proto";
type CreateRequest struct {
	Name   string `protobuf:"bytes,1,opt,name=Name,json=name" json:"Name,omitempty"`
	Access string `protobuf:"bytes,2,opt,name=Access,json=access" json:"Access,omitempty"`
}

func (m *CreateRequest) Reset()                    { *m = CreateRequest{} }
func (m *CreateRequest) String() string            { return proto.CompactTextString(m) }
func (*CreateRequest) ProtoMessage()               {}
func (*CreateRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *CreateRequest) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *CreateRequest) GetAccess() string {
	if m != nil {
		return m.Access
	}
	return ""
}

type CreateResponse struct {
	Certificate string `protobuf:"bytes,1,opt,name=Certificate,json=certificate" json:"Certificate,omitempty"`
}

func (m *CreateResponse) Reset()                    { *m = CreateResponse{} }
func (m *CreateResponse) String() string            { return proto.CompactTextString(m) }
func (*CreateResponse) ProtoMessage()               {}
func (*CreateResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *CreateResponse) GetCertificate() string {
	if m != nil {
		return m.Certificate
	}
	return ""
}

type PingRequest struct {
}

func (m *PingRequest) Reset()                    { *m = PingRequest{} }
func (m *PingRequest) String() string            { return proto.CompactTextString(m) }
func (*PingRequest) ProtoMessage()               {}
func (*PingRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

type PingResponse struct {
}

func (m *PingResponse) Reset()                    { *m = PingResponse{} }
func (m *PingResponse) String() string            { return proto.CompactTextString(m) }
func (*PingResponse) ProtoMessage()               {}
func (*PingResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func init() {
	proto.RegisterType((*CreateRequest)(nil), "rfccreator.CreateRequest")
	proto.RegisterType((*CreateResponse)(nil), "rfccreator.CreateResponse")
	proto.RegisterType((*PingRequest)(nil), "rfccreator.PingRequest")
	proto.RegisterType((*PingResponse)(nil), "rfccreator.PingResponse")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for RFCManager service

type RFCManagerClient interface {
	CreateRFC(ctx context.Context, in *CreateRequest, opts ...grpc.CallOption) (*CreateResponse, error)
}

type rFCManagerClient struct {
	cc *grpc.ClientConn
}

func NewRFCManagerClient(cc *grpc.ClientConn) RFCManagerClient {
	return &rFCManagerClient{cc}
}

func (c *rFCManagerClient) CreateRFC(ctx context.Context, in *CreateRequest, opts ...grpc.CallOption) (*CreateResponse, error) {
	out := new(CreateResponse)
	err := grpc.Invoke(ctx, "/rfccreator.RFCManager/CreateRFC", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for RFCManager service

type RFCManagerServer interface {
	CreateRFC(context.Context, *CreateRequest) (*CreateResponse, error)
}

func RegisterRFCManagerServer(s *grpc.Server, srv RFCManagerServer) {
	s.RegisterService(&_RFCManager_serviceDesc, srv)
}

func _RFCManager_CreateRFC_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RFCManagerServer).CreateRFC(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/rfccreator.RFCManager/CreateRFC",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RFCManagerServer).CreateRFC(ctx, req.(*CreateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _RFCManager_serviceDesc = grpc.ServiceDesc{
	ServiceName: "rfccreator.RFCManager",
	HandlerType: (*RFCManagerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateRFC",
			Handler:    _RFCManager_CreateRFC_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/softcat-rfc-creator.proto",
}

func init() { proto.RegisterFile("proto/softcat-rfc-creator.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 221 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0x6c, 0x4f, 0x4d, 0x4b, 0xc3, 0x40,
	0x10, 0xb5, 0x52, 0x02, 0x9d, 0xd8, 0x1e, 0xf6, 0x20, 0x35, 0x08, 0x96, 0x3d, 0x79, 0x69, 0x02,
	0xf5, 0xe8, 0x49, 0x22, 0xbd, 0x29, 0x92, 0x7f, 0x30, 0x0e, 0x93, 0xb0, 0x60, 0x77, 0xe2, 0xee,
	0xf8, 0xff, 0xc5, 0xdd, 0x94, 0x20, 0xf4, 0xf8, 0x3e, 0xe6, 0xbd, 0x79, 0xf0, 0x30, 0x06, 0x51,
	0x69, 0xa2, 0xf4, 0x4a, 0xa8, 0xfb, 0xd0, 0xd3, 0x9e, 0x02, 0xa3, 0x4a, 0xa8, 0x93, 0x62, 0x20,
	0xf4, 0x34, 0x31, 0xd5, 0xfd, 0x20, 0x32, 0x7c, 0x71, 0x83, 0xa3, 0x6b, 0xd0, 0x7b, 0x51, 0x54,
	0x27, 0x3e, 0x66, 0xa7, 0x7d, 0x86, 0x75, 0xfb, 0x67, 0xe4, 0x8e, 0xbf, 0x7f, 0x38, 0xaa, 0x31,
	0xb0, 0x7c, 0xc7, 0x13, 0x6f, 0x17, 0xbb, 0xc5, 0xe3, 0xaa, 0x5b, 0x7a, 0x3c, 0xb1, 0xb9, 0x85,
	0xe2, 0x85, 0x88, 0x63, 0xdc, 0x5e, 0x27, 0xb6, 0xc0, 0x84, 0xec, 0x01, 0x36, 0xe7, 0xe3, 0x38,
	0x8a, 0x8f, 0x6c, 0x76, 0x50, 0xb6, 0x1c, 0xd4, 0xf5, 0x8e, 0x50, 0xcf, 0x21, 0x25, 0xcd, 0x94,
	0x5d, 0x43, 0xf9, 0xe1, 0xfc, 0x30, 0xd5, 0xd9, 0x0d, 0xdc, 0x64, 0x98, 0x03, 0x0e, 0x1d, 0x40,
	0x77, 0x6c, 0xdf, 0xd0, 0xe3, 0xc0, 0xc1, 0xbc, 0xc2, 0x6a, 0x2a, 0x38, 0xb6, 0xe6, 0xae, 0x9e,
	0x57, 0xd5, 0xff, 0x9e, 0xae, 0xaa, 0x4b, 0x52, 0x4e, 0xb4, 0x57, 0x9f, 0x45, 0x9a, 0xfa, 0xf4,
	0x1b, 0x00, 0x00, 0xff, 0xff, 0xd3, 0x96, 0x31, 0x97, 0x37, 0x01, 0x00, 0x00,
}
