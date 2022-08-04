package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type Variable struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:",inline"`

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
	TypeMeta `json:",inline"`
	ListMeta `json:",inline"`

	Items []Variable `json:"items"`
}

func (m *VariableList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *VariableList) DeepCopyObject() runtime.Object {
	return m
}
