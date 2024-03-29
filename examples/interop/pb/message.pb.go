// Code generated by protoc-gen-go. DO NOT EDIT.
// source: message.proto

package pb

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type MessageBody struct {
	JSON                 string   `protobuf:"bytes,1,opt,name=JSON,proto3" json:"JSON,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *MessageBody) Reset()         { *m = MessageBody{} }
func (m *MessageBody) String() string { return proto.CompactTextString(m) }
func (*MessageBody) ProtoMessage()    {}
func (*MessageBody) Descriptor() ([]byte, []int) {
	return fileDescriptor_33c57e4bae7b9afd, []int{0}
}

func (m *MessageBody) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_MessageBody.Unmarshal(m, b)
}
func (m *MessageBody) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_MessageBody.Marshal(b, m, deterministic)
}
func (m *MessageBody) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MessageBody.Merge(m, src)
}
func (m *MessageBody) XXX_Size() int {
	return xxx_messageInfo_MessageBody.Size(m)
}
func (m *MessageBody) XXX_DiscardUnknown() {
	xxx_messageInfo_MessageBody.DiscardUnknown(m)
}

var xxx_messageInfo_MessageBody proto.InternalMessageInfo

func (m *MessageBody) GetJSON() string {
	if m != nil {
		return m.JSON
	}
	return ""
}

func init() {
	proto.RegisterType((*MessageBody)(nil), "MessageBody")
}

func init() { proto.RegisterFile("message.proto", fileDescriptor_33c57e4bae7b9afd) }

var fileDescriptor_33c57e4bae7b9afd = []byte{
	// 78 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0xcd, 0x4d, 0x2d, 0x2e,
	0x4e, 0x4c, 0x4f, 0xd5, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x57, 0x52, 0xe4, 0xe2, 0xf6, 0x85, 0x08,
	0x38, 0xe5, 0xa7, 0x54, 0x0a, 0x09, 0x71, 0xb1, 0x78, 0x05, 0xfb, 0xfb, 0x49, 0x30, 0x2a, 0x30,
	0x6a, 0x70, 0x06, 0x81, 0xd9, 0x4e, 0x2c, 0x51, 0x4c, 0x05, 0x49, 0x49, 0x6c, 0x60, 0xf5, 0xc6,
	0x80, 0x00, 0x00, 0x00, 0xff, 0xff, 0x5c, 0x23, 0xf7, 0x8d, 0x40, 0x00, 0x00, 0x00,
}
