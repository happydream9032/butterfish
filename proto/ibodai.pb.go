// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.29.0
// 	protoc        v3.21.12
// source: ibodai.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// The Command message is sent from the server to the client. The client
// should respond with a Response message with the same ID.
type Command struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id      string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Command string `protobuf:"bytes,2,opt,name=command,proto3" json:"command,omitempty"`
}

func (x *Command) Reset() {
	*x = Command{}
	if protoimpl.UnsafeEnabled {
		mi := &file_ibodai_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Command) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Command) ProtoMessage() {}

func (x *Command) ProtoReflect() protoreflect.Message {
	mi := &file_ibodai_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Command.ProtoReflect.Descriptor instead.
func (*Command) Descriptor() ([]byte, []int) {
	return file_ibodai_proto_rawDescGZIP(), []int{0}
}

func (x *Command) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Command) GetCommand() string {
	if x != nil {
		return x.Command
	}
	return ""
}

type ClientMessage struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Payload:
	//
	//	*ClientMessage_ClientHello
	//	*ClientMessage_CommandOutput
	//	*ClientMessage_CommandDone
	Payload isClientMessage_Payload `protobuf_oneof:"payload"`
}

func (x *ClientMessage) Reset() {
	*x = ClientMessage{}
	if protoimpl.UnsafeEnabled {
		mi := &file_ibodai_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ClientMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClientMessage) ProtoMessage() {}

func (x *ClientMessage) ProtoReflect() protoreflect.Message {
	mi := &file_ibodai_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ClientMessage.ProtoReflect.Descriptor instead.
func (*ClientMessage) Descriptor() ([]byte, []int) {
	return file_ibodai_proto_rawDescGZIP(), []int{1}
}

func (m *ClientMessage) GetPayload() isClientMessage_Payload {
	if m != nil {
		return m.Payload
	}
	return nil
}

func (x *ClientMessage) GetClientHello() *ClientHello {
	if x, ok := x.GetPayload().(*ClientMessage_ClientHello); ok {
		return x.ClientHello
	}
	return nil
}

func (x *ClientMessage) GetCommandOutput() *CommandOutput {
	if x, ok := x.GetPayload().(*ClientMessage_CommandOutput); ok {
		return x.CommandOutput
	}
	return nil
}

func (x *ClientMessage) GetCommandDone() *CommandDone {
	if x, ok := x.GetPayload().(*ClientMessage_CommandDone); ok {
		return x.CommandDone
	}
	return nil
}

type isClientMessage_Payload interface {
	isClientMessage_Payload()
}

type ClientMessage_ClientHello struct {
	ClientHello *ClientHello `protobuf:"bytes,1,opt,name=client_hello,json=clientHello,proto3,oneof"`
}

type ClientMessage_CommandOutput struct {
	CommandOutput *CommandOutput `protobuf:"bytes,2,opt,name=command_output,json=commandOutput,proto3,oneof"`
}

type ClientMessage_CommandDone struct {
	CommandDone *CommandDone `protobuf:"bytes,3,opt,name=command_done,json=commandDone,proto3,oneof"`
}

func (*ClientMessage_ClientHello) isClientMessage_Payload() {}

func (*ClientMessage_CommandOutput) isClientMessage_Payload() {}

func (*ClientMessage_CommandDone) isClientMessage_Payload() {}

// The CommandResult message is sent from the client to the server. The response
// should reference the ID of the command that it is responding to.
type CommandOutput struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CommandId     string `protobuf:"bytes,1,opt,name=command_id,json=commandId,proto3" json:"command_id,omitempty"`
	ResponseChunk []byte `protobuf:"bytes,2,opt,name=response_chunk,json=responseChunk,proto3" json:"response_chunk,omitempty"`
}

func (x *CommandOutput) Reset() {
	*x = CommandOutput{}
	if protoimpl.UnsafeEnabled {
		mi := &file_ibodai_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CommandOutput) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CommandOutput) ProtoMessage() {}

func (x *CommandOutput) ProtoReflect() protoreflect.Message {
	mi := &file_ibodai_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CommandOutput.ProtoReflect.Descriptor instead.
func (*CommandOutput) Descriptor() ([]byte, []int) {
	return file_ibodai_proto_rawDescGZIP(), []int{2}
}

func (x *CommandOutput) GetCommandId() string {
	if x != nil {
		return x.CommandId
	}
	return ""
}

func (x *CommandOutput) GetResponseChunk() []byte {
	if x != nil {
		return x.ResponseChunk
	}
	return nil
}

type CommandDone struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CommandId string `protobuf:"bytes,1,opt,name=command_id,json=commandId,proto3" json:"command_id,omitempty"`
	ExitCode  int32  `protobuf:"varint,2,opt,name=exit_code,json=exitCode,proto3" json:"exit_code,omitempty"`
}

func (x *CommandDone) Reset() {
	*x = CommandDone{}
	if protoimpl.UnsafeEnabled {
		mi := &file_ibodai_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CommandDone) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CommandDone) ProtoMessage() {}

func (x *CommandDone) ProtoReflect() protoreflect.Message {
	mi := &file_ibodai_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CommandDone.ProtoReflect.Descriptor instead.
func (*CommandDone) Descriptor() ([]byte, []int) {
	return file_ibodai_proto_rawDescGZIP(), []int{3}
}

func (x *CommandDone) GetCommandId() string {
	if x != nil {
		return x.CommandId
	}
	return ""
}

func (x *CommandDone) GetExitCode() int32 {
	if x != nil {
		return x.ExitCode
	}
	return 0
}

type ClientHello struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ClientToken string `protobuf:"bytes,1,opt,name=client_token,json=clientToken,proto3" json:"client_token,omitempty"`
}

func (x *ClientHello) Reset() {
	*x = ClientHello{}
	if protoimpl.UnsafeEnabled {
		mi := &file_ibodai_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ClientHello) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClientHello) ProtoMessage() {}

func (x *ClientHello) ProtoReflect() protoreflect.Message {
	mi := &file_ibodai_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ClientHello.ProtoReflect.Descriptor instead.
func (*ClientHello) Descriptor() ([]byte, []int) {
	return file_ibodai_proto_rawDescGZIP(), []int{4}
}

func (x *ClientHello) GetClientToken() string {
	if x != nil {
		return x.ClientToken
	}
	return ""
}

var File_ibodai_proto protoreflect.FileDescriptor

var file_ibodai_proto_rawDesc = []byte{
	0x0a, 0x0c, 0x69, 0x62, 0x6f, 0x64, 0x61, 0x69, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x33,
	0x0a, 0x07, 0x43, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x6f, 0x6d,
	0x6d, 0x61, 0x6e, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x63, 0x6f, 0x6d, 0x6d,
	0x61, 0x6e, 0x64, 0x22, 0xb9, 0x01, 0x0a, 0x0d, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x4d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x31, 0x0a, 0x0c, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x5f,
	0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0c, 0x2e, 0x43, 0x6c,
	0x69, 0x65, 0x6e, 0x74, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x48, 0x00, 0x52, 0x0b, 0x63, 0x6c, 0x69,
	0x65, 0x6e, 0x74, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x12, 0x37, 0x0a, 0x0e, 0x63, 0x6f, 0x6d, 0x6d,
	0x61, 0x6e, 0x64, 0x5f, 0x6f, 0x75, 0x74, 0x70, 0x75, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x0e, 0x2e, 0x43, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x4f, 0x75, 0x74, 0x70, 0x75, 0x74,
	0x48, 0x00, 0x52, 0x0d, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x4f, 0x75, 0x74, 0x70, 0x75,
	0x74, 0x12, 0x31, 0x0a, 0x0c, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x5f, 0x64, 0x6f, 0x6e,
	0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0c, 0x2e, 0x43, 0x6f, 0x6d, 0x6d, 0x61, 0x6e,
	0x64, 0x44, 0x6f, 0x6e, 0x65, 0x48, 0x00, 0x52, 0x0b, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64,
	0x44, 0x6f, 0x6e, 0x65, 0x42, 0x09, 0x0a, 0x07, 0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x22,
	0x55, 0x0a, 0x0d, 0x43, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x4f, 0x75, 0x74, 0x70, 0x75, 0x74,
	0x12, 0x1d, 0x0a, 0x0a, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x5f, 0x69, 0x64, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x49, 0x64, 0x12,
	0x25, 0x0a, 0x0e, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x5f, 0x63, 0x68, 0x75, 0x6e,
	0x6b, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0d, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x43, 0x68, 0x75, 0x6e, 0x6b, 0x22, 0x49, 0x0a, 0x0b, 0x43, 0x6f, 0x6d, 0x6d, 0x61, 0x6e,
	0x64, 0x44, 0x6f, 0x6e, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64,
	0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x63, 0x6f, 0x6d, 0x6d, 0x61,
	0x6e, 0x64, 0x49, 0x64, 0x12, 0x1b, 0x0a, 0x09, 0x65, 0x78, 0x69, 0x74, 0x5f, 0x63, 0x6f, 0x64,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x08, 0x65, 0x78, 0x69, 0x74, 0x43, 0x6f, 0x64,
	0x65, 0x22, 0x30, 0x0a, 0x0b, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x48, 0x65, 0x6c, 0x6c, 0x6f,
	0x12, 0x21, 0x0a, 0x0c, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x5f, 0x74, 0x6f, 0x6b, 0x65, 0x6e,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x54, 0x6f,
	0x6b, 0x65, 0x6e, 0x32, 0x30, 0x0a, 0x06, 0x49, 0x62, 0x6f, 0x64, 0x61, 0x69, 0x12, 0x26, 0x0a,
	0x06, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x12, 0x0e, 0x2e, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74,
	0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x1a, 0x08, 0x2e, 0x43, 0x6f, 0x6d, 0x6d, 0x61, 0x6e,
	0x64, 0x28, 0x01, 0x30, 0x01, 0x42, 0x23, 0x5a, 0x21, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x62, 0x61, 0x6b, 0x6b, 0x73, 0x2f, 0x62, 0x75, 0x74, 0x74, 0x65, 0x72,
	0x66, 0x69, 0x73, 0x68, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_ibodai_proto_rawDescOnce sync.Once
	file_ibodai_proto_rawDescData = file_ibodai_proto_rawDesc
)

func file_ibodai_proto_rawDescGZIP() []byte {
	file_ibodai_proto_rawDescOnce.Do(func() {
		file_ibodai_proto_rawDescData = protoimpl.X.CompressGZIP(file_ibodai_proto_rawDescData)
	})
	return file_ibodai_proto_rawDescData
}

var file_ibodai_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_ibodai_proto_goTypes = []interface{}{
	(*Command)(nil),       // 0: Command
	(*ClientMessage)(nil), // 1: ClientMessage
	(*CommandOutput)(nil), // 2: CommandOutput
	(*CommandDone)(nil),   // 3: CommandDone
	(*ClientHello)(nil),   // 4: ClientHello
}
var file_ibodai_proto_depIdxs = []int32{
	4, // 0: ClientMessage.client_hello:type_name -> ClientHello
	2, // 1: ClientMessage.command_output:type_name -> CommandOutput
	3, // 2: ClientMessage.command_done:type_name -> CommandDone
	1, // 3: Ibodai.Stream:input_type -> ClientMessage
	0, // 4: Ibodai.Stream:output_type -> Command
	4, // [4:5] is the sub-list for method output_type
	3, // [3:4] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_ibodai_proto_init() }
func file_ibodai_proto_init() {
	if File_ibodai_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_ibodai_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Command); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_ibodai_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ClientMessage); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_ibodai_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CommandOutput); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_ibodai_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CommandDone); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_ibodai_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ClientHello); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_ibodai_proto_msgTypes[1].OneofWrappers = []interface{}{
		(*ClientMessage_ClientHello)(nil),
		(*ClientMessage_CommandOutput)(nil),
		(*ClientMessage_CommandDone)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_ibodai_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_ibodai_proto_goTypes,
		DependencyIndexes: file_ibodai_proto_depIdxs,
		MessageInfos:      file_ibodai_proto_msgTypes,
	}.Build()
	File_ibodai_proto = out.File
	file_ibodai_proto_rawDesc = nil
	file_ibodai_proto_goTypes = nil
	file_ibodai_proto_depIdxs = nil
}
