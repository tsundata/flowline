package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type Function struct {
	TypeMeta
	ObjectMeta

	Runtime string `json:"runtime,omitempty"`
	Code    string `json:"code,omitempty"`
}

func (m *Function) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Function) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}

type FunctionList struct {
	TypeMeta
	ListMeta
	Items []Function
}

func (m *FunctionList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *FunctionList) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}
