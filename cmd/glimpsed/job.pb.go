// Code generated by protoc-gen-go.
// source: job.proto
// DO NOT EDIT!

package main

import proto "code.google.com/p/goprotobuf/proto"
import json "encoding/json"
import math "math"

// Reference proto, json, and math imports to suppress error if they are not otherwise used.
var _ = proto.Marshal
var _ = &json.SyntaxError{}
var _ = math.Inf

type Endpoint struct {
	Name             *string `protobuf:"bytes,1,req,name=name" json:"name,omitempty"`
	Host             *string `protobuf:"bytes,2,opt,name=host" json:"host,omitempty"`
	Port             *uint32 `protobuf:"varint,3,opt,name=port" json:"port,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *Endpoint) Reset()         { *m = Endpoint{} }
func (m *Endpoint) String() string { return proto.CompactTextString(m) }
func (*Endpoint) ProtoMessage()    {}

func (m *Endpoint) GetName() string {
	if m != nil && m.Name != nil {
		return *m.Name
	}
	return ""
}

func (m *Endpoint) GetHost() string {
	if m != nil && m.Host != nil {
		return *m.Host
	}
	return ""
}

func (m *Endpoint) GetPort() uint32 {
	if m != nil && m.Port != nil {
		return *m.Port
	}
	return 0
}

type Instance struct {
	Index            *uint32     `protobuf:"varint,1,req,name=index" json:"index,omitempty"`
	Endpoint         []*Endpoint `protobuf:"bytes,2,rep,name=endpoint" json:"endpoint,omitempty"`
	XXX_unrecognized []byte      `json:"-"`
}

func (m *Instance) Reset()         { *m = Instance{} }
func (m *Instance) String() string { return proto.CompactTextString(m) }
func (*Instance) ProtoMessage()    {}

func (m *Instance) GetIndex() uint32 {
	if m != nil && m.Index != nil {
		return *m.Index
	}
	return 0
}

func (m *Instance) GetEndpoint() []*Endpoint {
	if m != nil {
		return m.Endpoint
	}
	return nil
}

type Job struct {
	Zone             *string     `protobuf:"bytes,1,opt,name=zone" json:"zone,omitempty"`
	Product          *string     `protobuf:"bytes,2,opt,name=product" json:"product,omitempty"`
	Env              *string     `protobuf:"bytes,3,opt,name=env" json:"env,omitempty"`
	Name             *string     `protobuf:"bytes,4,opt,name=name" json:"name,omitempty"`
	Instance         []*Instance `protobuf:"bytes,5,rep,name=instance" json:"instance,omitempty"`
	XXX_unrecognized []byte      `json:"-"`
}

func (m *Job) Reset()         { *m = Job{} }
func (m *Job) String() string { return proto.CompactTextString(m) }
func (*Job) ProtoMessage()    {}

func (m *Job) GetZone() string {
	if m != nil && m.Zone != nil {
		return *m.Zone
	}
	return ""
}

func (m *Job) GetProduct() string {
	if m != nil && m.Product != nil {
		return *m.Product
	}
	return ""
}

func (m *Job) GetEnv() string {
	if m != nil && m.Env != nil {
		return *m.Env
	}
	return ""
}

func (m *Job) GetName() string {
	if m != nil && m.Name != nil {
		return *m.Name
	}
	return ""
}

func (m *Job) GetInstance() []*Instance {
	if m != nil {
		return m.Instance
	}
	return nil
}

func init() {
}