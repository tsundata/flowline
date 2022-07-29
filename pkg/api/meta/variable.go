package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type Variable struct {
	TypeMeta
	ObjectMeta

	Value    string `json:"value,omitempty"`
	Describe string `json:"describe,omitempty"`
}

func (m *Variable) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Variable) DeepCopyObject() runtime.Object {
	return m
}

type VariableList struct {
	TypeMeta
	ListMeta
	Items []Variable
}

func (m *VariableList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *VariableList) DeepCopyObject() runtime.Object {
	return m
}
