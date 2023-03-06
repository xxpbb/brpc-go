// Code generated by protoc-gen-go-brpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-brpc v0.1.0
// - protoc             v3.21.12
// source: echo.proto

package echo

import (
	context "context"
	brpc_go "github.com/xxpbb/brpc-go"
	status "github.com/xxpbb/brpc-go/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the brpc package it is being compiled against.
// Requires bRPC-Go v0.1.0 or later.
const _ = brpc_go.SupportPackageIsVersion1

const (
	EchoService_Echo_FullMethodName = "/example.EchoService/Echo"
)

// EchoServiceClient is the client API for EchoService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type EchoServiceClient interface {
	Echo(ctx context.Context, in *EchoRequest, opts ...brpc_go.CallOption) (*EchoResponse, error)
}

type echoServiceClient struct {
	cc brpc_go.ClientConnInterface
}

func NewEchoServiceClient(cc brpc_go.ClientConnInterface) EchoServiceClient {
	return &echoServiceClient{cc}
}

func (c *echoServiceClient) Echo(ctx context.Context, in *EchoRequest, opts ...brpc_go.CallOption) (*EchoResponse, error) {
	out := new(EchoResponse)
	err := c.cc.Invoke(ctx, EchoService_Echo_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// EchoServiceServer is the server API for EchoService service.
// All implementations must embed UnimplementedEchoServiceServer
// for forward compatibility
type EchoServiceServer interface {
	Echo(context.Context, *EchoRequest) (*EchoResponse, error)
	mustEmbedUnimplementedEchoServiceServer()
}

// UnimplementedEchoServiceServer must be embedded to have forward compatible implementations.
type UnimplementedEchoServiceServer struct {
}

func (UnimplementedEchoServiceServer) Echo(context.Context, *EchoRequest) (*EchoResponse, error) {
	return nil, status.Errorf(-1, "method Echo not implemented")
}
func (UnimplementedEchoServiceServer) mustEmbedUnimplementedEchoServiceServer() {}

// UnsafeEchoServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to EchoServiceServer will
// result in compilation errors.
type UnsafeEchoServiceServer interface {
	mustEmbedUnimplementedEchoServiceServer()
}

func RegisterEchoServiceServer(s brpc_go.ServiceRegistrar, srv EchoServiceServer) {
	s.RegisterService(&EchoService_ServiceDesc, srv)
}

func _EchoService_Echo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error) (interface{}, error) {
	in := new(EchoRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	return srv.(EchoServiceServer).Echo(ctx, in)
}

// EchoService_ServiceDesc is the brpc_go.ServiceDesc for EchoService service.
// It's only intended for direct use with brpc_go.RegisterService,
// and not to be introspected or modified (even as a copy)
var EchoService_ServiceDesc = brpc_go.ServiceDesc{
	ServiceName: "example.EchoService",
	HandlerType: (*EchoServiceServer)(nil),
	Methods: []brpc_go.MethodDesc{
		{
			MethodName: "Echo",
			Handler:    _EchoService_Echo_Handler,
		},
	},
}
