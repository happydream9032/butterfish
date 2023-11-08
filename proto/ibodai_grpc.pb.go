// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.25.0
// source: ibodai.proto

package proto

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	Ibodai_Stream_FullMethodName = "/Ibodai/Stream"
)

// IbodaiClient is the client API for Ibodai service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type IbodaiClient interface {
	Stream(ctx context.Context, opts ...grpc.CallOption) (Ibodai_StreamClient, error)
}

type ibodaiClient struct {
	cc grpc.ClientConnInterface
}

func NewIbodaiClient(cc grpc.ClientConnInterface) IbodaiClient {
	return &ibodaiClient{cc}
}

func (c *ibodaiClient) Stream(ctx context.Context, opts ...grpc.CallOption) (Ibodai_StreamClient, error) {
	stream, err := c.cc.NewStream(ctx, &Ibodai_ServiceDesc.Streams[0], Ibodai_Stream_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &ibodaiStreamClient{stream}
	return x, nil
}

type Ibodai_StreamClient interface {
	Send(*ClientMessage) error
	Recv() (*Command, error)
	grpc.ClientStream
}

type ibodaiStreamClient struct {
	grpc.ClientStream
}

func (x *ibodaiStreamClient) Send(m *ClientMessage) error {
	return x.ClientStream.SendMsg(m)
}

func (x *ibodaiStreamClient) Recv() (*Command, error) {
	m := new(Command)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// IbodaiServer is the server API for Ibodai service.
// All implementations must embed UnimplementedIbodaiServer
// for forward compatibility
type IbodaiServer interface {
	Stream(Ibodai_StreamServer) error
	mustEmbedUnimplementedIbodaiServer()
}

// UnimplementedIbodaiServer must be embedded to have forward compatible implementations.
type UnimplementedIbodaiServer struct {
}

func (UnimplementedIbodaiServer) Stream(Ibodai_StreamServer) error {
	return status.Errorf(codes.Unimplemented, "method Stream not implemented")
}
func (UnimplementedIbodaiServer) mustEmbedUnimplementedIbodaiServer() {}

// UnsafeIbodaiServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to IbodaiServer will
// result in compilation errors.
type UnsafeIbodaiServer interface {
	mustEmbedUnimplementedIbodaiServer()
}

func RegisterIbodaiServer(s grpc.ServiceRegistrar, srv IbodaiServer) {
	s.RegisterService(&Ibodai_ServiceDesc, srv)
}

func _Ibodai_Stream_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(IbodaiServer).Stream(&ibodaiStreamServer{stream})
}

type Ibodai_StreamServer interface {
	Send(*Command) error
	Recv() (*ClientMessage, error)
	grpc.ServerStream
}

type ibodaiStreamServer struct {
	grpc.ServerStream
}

func (x *ibodaiStreamServer) Send(m *Command) error {
	return x.ServerStream.SendMsg(m)
}

func (x *ibodaiStreamServer) Recv() (*ClientMessage, error) {
	m := new(ClientMessage)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Ibodai_ServiceDesc is the grpc.ServiceDesc for Ibodai service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Ibodai_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "Ibodai",
	HandlerType: (*IbodaiServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Stream",
			Handler:       _Ibodai_Stream_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "ibodai.proto",
}
