// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        (unknown)
// source: j5/config/v1/lock.proto

package config_j5pb

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

type LockFile struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Inputs  []*InputLock  `protobuf:"bytes,1,rep,name=inputs,proto3" json:"inputs,omitempty"`
	Plugins []*PluginLock `protobuf:"bytes,2,rep,name=plugins,proto3" json:"plugins,omitempty"`
}

func (x *LockFile) Reset() {
	*x = LockFile{}
	if protoimpl.UnsafeEnabled {
		mi := &file_j5_config_v1_lock_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LockFile) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LockFile) ProtoMessage() {}

func (x *LockFile) ProtoReflect() protoreflect.Message {
	mi := &file_j5_config_v1_lock_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LockFile.ProtoReflect.Descriptor instead.
func (*LockFile) Descriptor() ([]byte, []int) {
	return file_j5_config_v1_lock_proto_rawDescGZIP(), []int{0}
}

func (x *LockFile) GetInputs() []*InputLock {
	if x != nil {
		return x.Inputs
	}
	return nil
}

func (x *LockFile) GetPlugins() []*PluginLock {
	if x != nil {
		return x.Plugins
	}
	return nil
}

type InputLock struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name    string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Version string `protobuf:"bytes,2,opt,name=version,proto3" json:"version,omitempty"`
}

func (x *InputLock) Reset() {
	*x = InputLock{}
	if protoimpl.UnsafeEnabled {
		mi := &file_j5_config_v1_lock_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *InputLock) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InputLock) ProtoMessage() {}

func (x *InputLock) ProtoReflect() protoreflect.Message {
	mi := &file_j5_config_v1_lock_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InputLock.ProtoReflect.Descriptor instead.
func (*InputLock) Descriptor() ([]byte, []int) {
	return file_j5_config_v1_lock_proto_rawDescGZIP(), []int{1}
}

func (x *InputLock) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *InputLock) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

type PluginLock struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name    string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Version string `protobuf:"bytes,2,opt,name=version,proto3" json:"version,omitempty"`
}

func (x *PluginLock) Reset() {
	*x = PluginLock{}
	if protoimpl.UnsafeEnabled {
		mi := &file_j5_config_v1_lock_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PluginLock) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PluginLock) ProtoMessage() {}

func (x *PluginLock) ProtoReflect() protoreflect.Message {
	mi := &file_j5_config_v1_lock_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PluginLock.ProtoReflect.Descriptor instead.
func (*PluginLock) Descriptor() ([]byte, []int) {
	return file_j5_config_v1_lock_proto_rawDescGZIP(), []int{2}
}

func (x *PluginLock) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *PluginLock) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

var File_j5_config_v1_lock_proto protoreflect.FileDescriptor

var file_j5_config_v1_lock_proto_rawDesc = []byte{
	0x0a, 0x17, 0x6a, 0x35, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2f, 0x76, 0x31, 0x2f, 0x6c,
	0x6f, 0x63, 0x6b, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0c, 0x6a, 0x35, 0x2e, 0x63, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x22, 0x6f, 0x0a, 0x08, 0x4c, 0x6f, 0x63, 0x6b, 0x46,
	0x69, 0x6c, 0x65, 0x12, 0x2f, 0x0a, 0x06, 0x69, 0x6e, 0x70, 0x75, 0x74, 0x73, 0x18, 0x01, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x6a, 0x35, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e,
	0x76, 0x31, 0x2e, 0x49, 0x6e, 0x70, 0x75, 0x74, 0x4c, 0x6f, 0x63, 0x6b, 0x52, 0x06, 0x69, 0x6e,
	0x70, 0x75, 0x74, 0x73, 0x12, 0x32, 0x0a, 0x07, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x73, 0x18,
	0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x18, 0x2e, 0x6a, 0x35, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x2e, 0x76, 0x31, 0x2e, 0x50, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x4c, 0x6f, 0x63, 0x6b, 0x52,
	0x07, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x73, 0x22, 0x39, 0x0a, 0x09, 0x49, 0x6e, 0x70, 0x75,
	0x74, 0x4c, 0x6f, 0x63, 0x6b, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x76, 0x65, 0x72,
	0x73, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73,
	0x69, 0x6f, 0x6e, 0x22, 0x3a, 0x0a, 0x0a, 0x50, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x4c, 0x6f, 0x63,
	0x6b, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x42,
	0x39, 0x5a, 0x37, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x65,
	0x6e, 0x74, 0x6f, 0x70, 0x73, 0x2f, 0x6a, 0x35, 0x62, 0x75, 0x69, 0x6c, 0x64, 0x2f, 0x67, 0x65,
	0x6e, 0x2f, 0x6a, 0x35, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2f, 0x76, 0x31, 0x2f, 0x63,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x5f, 0x6a, 0x35, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_j5_config_v1_lock_proto_rawDescOnce sync.Once
	file_j5_config_v1_lock_proto_rawDescData = file_j5_config_v1_lock_proto_rawDesc
)

func file_j5_config_v1_lock_proto_rawDescGZIP() []byte {
	file_j5_config_v1_lock_proto_rawDescOnce.Do(func() {
		file_j5_config_v1_lock_proto_rawDescData = protoimpl.X.CompressGZIP(file_j5_config_v1_lock_proto_rawDescData)
	})
	return file_j5_config_v1_lock_proto_rawDescData
}

var file_j5_config_v1_lock_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_j5_config_v1_lock_proto_goTypes = []any{
	(*LockFile)(nil),   // 0: j5.config.v1.LockFile
	(*InputLock)(nil),  // 1: j5.config.v1.InputLock
	(*PluginLock)(nil), // 2: j5.config.v1.PluginLock
}
var file_j5_config_v1_lock_proto_depIdxs = []int32{
	1, // 0: j5.config.v1.LockFile.inputs:type_name -> j5.config.v1.InputLock
	2, // 1: j5.config.v1.LockFile.plugins:type_name -> j5.config.v1.PluginLock
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_j5_config_v1_lock_proto_init() }
func file_j5_config_v1_lock_proto_init() {
	if File_j5_config_v1_lock_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_j5_config_v1_lock_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*LockFile); i {
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
		file_j5_config_v1_lock_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*InputLock); i {
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
		file_j5_config_v1_lock_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*PluginLock); i {
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
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_j5_config_v1_lock_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_j5_config_v1_lock_proto_goTypes,
		DependencyIndexes: file_j5_config_v1_lock_proto_depIdxs,
		MessageInfos:      file_j5_config_v1_lock_proto_msgTypes,
	}.Build()
	File_j5_config_v1_lock_proto = out.File
	file_j5_config_v1_lock_proto_rawDesc = nil
	file_j5_config_v1_lock_proto_goTypes = nil
	file_j5_config_v1_lock_proto_depIdxs = nil
}