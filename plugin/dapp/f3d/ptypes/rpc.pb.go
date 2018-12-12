// Code generated by protoc-gen-go. DO NOT EDIT.
// source: rpc.proto

package types

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

type GameStartReq struct {
	Round int64 `protobuf:"varint,1,opt,name=round" json:"round,omitempty"`
}

func (m *GameStartReq) Reset()                    { *m = GameStartReq{} }
func (m *GameStartReq) String() string            { return proto.CompactTextString(m) }
func (*GameStartReq) ProtoMessage()               {}
func (*GameStartReq) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{0} }

func (m *GameStartReq) GetRound() int64 {
	if m != nil {
		return m.Round
	}
	return 0
}

type GameDrawReq struct {
	Round int64 `protobuf:"varint,1,opt,name=round" json:"round,omitempty"`
}

func (m *GameDrawReq) Reset()                    { *m = GameDrawReq{} }
func (m *GameDrawReq) String() string            { return proto.CompactTextString(m) }
func (*GameDrawReq) ProtoMessage()               {}
func (*GameDrawReq) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{1} }

func (m *GameDrawReq) GetRound() int64 {
	if m != nil {
		return m.Round
	}
	return 0
}

type KeyInfoQueryReq struct {
	Addr  string `protobuf:"bytes,1,opt,name=addr" json:"addr,omitempty"`
	Round int64  `protobuf:"varint,2,opt,name=round" json:"round,omitempty"`
}

func (m *KeyInfoQueryReq) Reset()                    { *m = KeyInfoQueryReq{} }
func (m *KeyInfoQueryReq) String() string            { return proto.CompactTextString(m) }
func (*KeyInfoQueryReq) ProtoMessage()               {}
func (*KeyInfoQueryReq) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{2} }

func (m *KeyInfoQueryReq) GetAddr() string {
	if m != nil {
		return m.Addr
	}
	return ""
}

func (m *KeyInfoQueryReq) GetRound() int64 {
	if m != nil {
		return m.Round
	}
	return 0
}

type RoundInfoQueryReq struct {
	Round int64 `protobuf:"varint,1,opt,name=round" json:"round,omitempty"`
}

func (m *RoundInfoQueryReq) Reset()                    { *m = RoundInfoQueryReq{} }
func (m *RoundInfoQueryReq) String() string            { return proto.CompactTextString(m) }
func (*RoundInfoQueryReq) ProtoMessage()               {}
func (*RoundInfoQueryReq) Descriptor() ([]byte, []int) { return fileDescriptor1, []int{3} }

func (m *RoundInfoQueryReq) GetRound() int64 {
	if m != nil {
		return m.Round
	}
	return 0
}

func init() {
	proto.RegisterType((*GameStartReq)(nil), "types.GameStartReq")
	proto.RegisterType((*GameDrawReq)(nil), "types.GameDrawReq")
	proto.RegisterType((*KeyInfoQueryReq)(nil), "types.KeyInfoQueryReq")
	proto.RegisterType((*RoundInfoQueryReq)(nil), "types.RoundInfoQueryReq")
}

func init() { proto.RegisterFile("rpc.proto", fileDescriptor1) }

var fileDescriptor1 = []byte{
	// 138 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x2c, 0x2a, 0x48, 0xd6,
	0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x2d, 0xa9, 0x2c, 0x48, 0x2d, 0x56, 0x52, 0xe1, 0xe2,
	0x71, 0x4f, 0xcc, 0x4d, 0x0d, 0x2e, 0x49, 0x2c, 0x2a, 0x09, 0x4a, 0x2d, 0x14, 0x12, 0xe1, 0x62,
	0x2d, 0xca, 0x2f, 0xcd, 0x4b, 0x91, 0x60, 0x54, 0x60, 0xd4, 0x60, 0x0e, 0x82, 0x70, 0x94, 0x94,
	0xb9, 0xb8, 0x41, 0xaa, 0x5c, 0x8a, 0x12, 0xcb, 0x71, 0x2b, 0xb2, 0xe6, 0xe2, 0xf7, 0x4e, 0xad,
	0xf4, 0xcc, 0x4b, 0xcb, 0x0f, 0x2c, 0x4d, 0x2d, 0xaa, 0x04, 0x29, 0x14, 0xe2, 0x62, 0x49, 0x4c,
	0x49, 0x29, 0x02, 0xab, 0xe3, 0x0c, 0x02, 0xb3, 0x11, 0x9a, 0x99, 0x90, 0x35, 0x6b, 0x72, 0x09,
	0x06, 0x81, 0x18, 0x28, 0xda, 0xb1, 0xda, 0x93, 0xc4, 0x06, 0xf6, 0x80, 0x31, 0x20, 0x00, 0x00,
	0xff, 0xff, 0x59, 0x93, 0x27, 0x1f, 0xcd, 0x00, 0x00, 0x00,
}
