// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        (unknown)
// source: test/v1/foo.proto

package test_pb

import (
	bcl_j5pb "github.com/pentops/bcl.go/gen/j5/bcl/v1/bcl_j5pb"
	_ "github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	_ "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type File struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Elements       []*Element               `protobuf:"bytes,3,rep,name=elements,proto3" json:"elements,omitempty"`
	SourceLocation *bcl_j5pb.SourceLocation `protobuf:"bytes,10,opt,name=source_location,json=sourceLocation,proto3" json:"source_location,omitempty"`
	SString        string                   `protobuf:"bytes,11,opt,name=s_string,json=sString,proto3" json:"s_string,omitempty"`
	RString        []string                 `protobuf:"bytes,12,rep,name=r_string,json=rString,proto3" json:"r_string,omitempty"`
	Tags           map[string]string        `protobuf:"bytes,13,rep,name=tags,proto3" json:"tags,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *File) Reset() {
	*x = File{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_v1_foo_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *File) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*File) ProtoMessage() {}

func (x *File) ProtoReflect() protoreflect.Message {
	mi := &file_test_v1_foo_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use File.ProtoReflect.Descriptor instead.
func (*File) Descriptor() ([]byte, []int) {
	return file_test_v1_foo_proto_rawDescGZIP(), []int{0}
}

func (x *File) GetElements() []*Element {
	if x != nil {
		return x.Elements
	}
	return nil
}

func (x *File) GetSourceLocation() *bcl_j5pb.SourceLocation {
	if x != nil {
		return x.SourceLocation
	}
	return nil
}

func (x *File) GetSString() string {
	if x != nil {
		return x.SString
	}
	return ""
}

func (x *File) GetRString() []string {
	if x != nil {
		return x.RString
	}
	return nil
}

func (x *File) GetTags() map[string]string {
	if x != nil {
		return x.Tags
	}
	return nil
}

type Element struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Type:
	//
	//	*Element_Foo_
	//	*Element_Bar_
	Type isElement_Type `protobuf_oneof:"type"`
}

func (x *Element) Reset() {
	*x = Element{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_v1_foo_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Element) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Element) ProtoMessage() {}

func (x *Element) ProtoReflect() protoreflect.Message {
	mi := &file_test_v1_foo_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Element.ProtoReflect.Descriptor instead.
func (*Element) Descriptor() ([]byte, []int) {
	return file_test_v1_foo_proto_rawDescGZIP(), []int{1}
}

func (m *Element) GetType() isElement_Type {
	if m != nil {
		return m.Type
	}
	return nil
}

func (x *Element) GetFoo() *Element_Foo {
	if x, ok := x.GetType().(*Element_Foo_); ok {
		return x.Foo
	}
	return nil
}

func (x *Element) GetBar() *Element_Bar {
	if x, ok := x.GetType().(*Element_Bar_); ok {
		return x.Bar
	}
	return nil
}

type isElement_Type interface {
	isElement_Type()
}

type Element_Foo_ struct {
	Foo *Element_Foo `protobuf:"bytes,1,opt,name=foo,proto3,oneof"`
}

type Element_Bar_ struct {
	Bar *Element_Bar `protobuf:"bytes,2,opt,name=bar,proto3,oneof"`
}

func (*Element_Foo_) isElement_Type() {}

func (*Element_Bar_) isElement_Type() {}

type Element_Foo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name        string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Description string `protobuf:"bytes,2,opt,name=description,proto3" json:"description,omitempty"`
}

func (x *Element_Foo) Reset() {
	*x = Element_Foo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_v1_foo_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Element_Foo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Element_Foo) ProtoMessage() {}

func (x *Element_Foo) ProtoReflect() protoreflect.Message {
	mi := &file_test_v1_foo_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Element_Foo.ProtoReflect.Descriptor instead.
func (*Element_Foo) Descriptor() ([]byte, []int) {
	return file_test_v1_foo_proto_rawDescGZIP(), []int{1, 0}
}

func (x *Element_Foo) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Element_Foo) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

type Element_Bar struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *Element_Bar) Reset() {
	*x = Element_Bar{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_v1_foo_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Element_Bar) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Element_Bar) ProtoMessage() {}

func (x *Element_Bar) ProtoReflect() protoreflect.Message {
	mi := &file_test_v1_foo_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Element_Bar.ProtoReflect.Descriptor instead.
func (*Element_Bar) Descriptor() ([]byte, []int) {
	return file_test_v1_foo_proto_rawDescGZIP(), []int{1, 1}
}

func (x *Element_Bar) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

var File_test_v1_foo_proto protoreflect.FileDescriptor

var file_test_v1_foo_proto_rawDesc = []byte{
	0x0a, 0x11, 0x74, 0x65, 0x73, 0x74, 0x2f, 0x76, 0x31, 0x2f, 0x66, 0x6f, 0x6f, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x07, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x76, 0x31, 0x1a, 0x1f, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1b, 0x6a,
	0x35, 0x2f, 0x62, 0x63, 0x6c, 0x2f, 0x76, 0x31, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1b, 0x6a, 0x35, 0x2f, 0x65,
	0x78, 0x74, 0x2f, 0x76, 0x31, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xa3, 0x02, 0x0a, 0x04, 0x46, 0x69, 0x6c, 0x65,
	0x12, 0x2c, 0x0a, 0x08, 0x65, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x73, 0x18, 0x03, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x10, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x45, 0x6c, 0x65,
	0x6d, 0x65, 0x6e, 0x74, 0x52, 0x08, 0x65, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x73, 0x12, 0x42,
	0x0a, 0x0f, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x5f, 0x6c, 0x6f, 0x63, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x6a, 0x35, 0x2e, 0x62, 0x63, 0x6c,
	0x2e, 0x76, 0x31, 0x2e, 0x53, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x4c, 0x6f, 0x63, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x52, 0x0e, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x4c, 0x6f, 0x63, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x12, 0x19, 0x0a, 0x08, 0x73, 0x5f, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x18, 0x0b,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x73, 0x53, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x12, 0x19, 0x0a,
	0x08, 0x72, 0x5f, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x18, 0x0c, 0x20, 0x03, 0x28, 0x09, 0x52,
	0x07, 0x72, 0x53, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x12, 0x3a, 0x0a, 0x04, 0x74, 0x61, 0x67, 0x73,
	0x18, 0x0d, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x76, 0x31,
	0x2e, 0x46, 0x69, 0x6c, 0x65, 0x2e, 0x54, 0x61, 0x67, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x42,
	0x0d, 0xc2, 0xff, 0x8e, 0x02, 0x08, 0xa2, 0x01, 0x05, 0x1a, 0x03, 0x74, 0x61, 0x67, 0x52, 0x04,
	0x74, 0x61, 0x67, 0x73, 0x1a, 0x37, 0x0a, 0x09, 0x54, 0x61, 0x67, 0x73, 0x45, 0x6e, 0x74, 0x72,
	0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03,
	0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0xbd, 0x01,
	0x0a, 0x07, 0x45, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x12, 0x28, 0x0a, 0x03, 0x66, 0x6f, 0x6f,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x76, 0x31,
	0x2e, 0x45, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x2e, 0x46, 0x6f, 0x6f, 0x48, 0x00, 0x52, 0x03,
	0x66, 0x6f, 0x6f, 0x12, 0x28, 0x0a, 0x03, 0x62, 0x61, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x14, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x45, 0x6c, 0x65, 0x6d, 0x65,
	0x6e, 0x74, 0x2e, 0x42, 0x61, 0x72, 0x48, 0x00, 0x52, 0x03, 0x62, 0x61, 0x72, 0x1a, 0x3b, 0x0a,
	0x03, 0x46, 0x6f, 0x6f, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73, 0x63,
	0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64,
	0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x1a, 0x19, 0x0a, 0x03, 0x42, 0x61,
	0x72, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x04, 0x6e, 0x61, 0x6d, 0x65, 0x42, 0x06, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x42, 0x2f, 0x5a,
	0x2d, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x65, 0x6e, 0x74,
	0x6f, 0x70, 0x73, 0x2f, 0x62, 0x63, 0x6c, 0x2e, 0x67, 0x6f, 0x2f, 0x67, 0x65, 0x6e, 0x2f, 0x74,
	0x65, 0x73, 0x74, 0x2f, 0x76, 0x31, 0x2f, 0x74, 0x65, 0x73, 0x74, 0x5f, 0x70, 0x62, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_test_v1_foo_proto_rawDescOnce sync.Once
	file_test_v1_foo_proto_rawDescData = file_test_v1_foo_proto_rawDesc
)

func file_test_v1_foo_proto_rawDescGZIP() []byte {
	file_test_v1_foo_proto_rawDescOnce.Do(func() {
		file_test_v1_foo_proto_rawDescData = protoimpl.X.CompressGZIP(file_test_v1_foo_proto_rawDescData)
	})
	return file_test_v1_foo_proto_rawDescData
}

var file_test_v1_foo_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_test_v1_foo_proto_goTypes = []any{
	(*File)(nil),                    // 0: test.v1.File
	(*Element)(nil),                 // 1: test.v1.Element
	nil,                             // 2: test.v1.File.TagsEntry
	(*Element_Foo)(nil),             // 3: test.v1.Element.Foo
	(*Element_Bar)(nil),             // 4: test.v1.Element.Bar
	(*bcl_j5pb.SourceLocation)(nil), // 5: j5.bcl.v1.SourceLocation
}
var file_test_v1_foo_proto_depIdxs = []int32{
	1, // 0: test.v1.File.elements:type_name -> test.v1.Element
	5, // 1: test.v1.File.source_location:type_name -> j5.bcl.v1.SourceLocation
	2, // 2: test.v1.File.tags:type_name -> test.v1.File.TagsEntry
	3, // 3: test.v1.Element.foo:type_name -> test.v1.Element.Foo
	4, // 4: test.v1.Element.bar:type_name -> test.v1.Element.Bar
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_test_v1_foo_proto_init() }
func file_test_v1_foo_proto_init() {
	if File_test_v1_foo_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_test_v1_foo_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*File); i {
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
		file_test_v1_foo_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*Element); i {
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
		file_test_v1_foo_proto_msgTypes[3].Exporter = func(v any, i int) any {
			switch v := v.(*Element_Foo); i {
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
		file_test_v1_foo_proto_msgTypes[4].Exporter = func(v any, i int) any {
			switch v := v.(*Element_Bar); i {
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
	file_test_v1_foo_proto_msgTypes[1].OneofWrappers = []any{
		(*Element_Foo_)(nil),
		(*Element_Bar_)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_test_v1_foo_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_test_v1_foo_proto_goTypes,
		DependencyIndexes: file_test_v1_foo_proto_depIdxs,
		MessageInfos:      file_test_v1_foo_proto_msgTypes,
	}.Build()
	File_test_v1_foo_proto = out.File
	file_test_v1_foo_proto_rawDesc = nil
	file_test_v1_foo_proto_goTypes = nil
	file_test_v1_foo_proto_depIdxs = nil
}
