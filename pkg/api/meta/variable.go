package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type Variable struct {
	TypeMeta
	ObjectMeta

	Key         string `json:"key,omitempty"`
	Value       string `json:"value,omitempty"`
	Description string `json:"description,omitempty"`
}

func (m *Variable) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Variable) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}

type VariableList struct {
	TypeMeta
	ListMeta
	Items []User
}

func (m *VariableList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *VariableList) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}
