package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type Code struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:",inline"`

	Describe string `json:"describe,omitempty"`
	Runtime  string `json:"runtime,omitempty"`
	Code     string `json:"code,omitempty"`
}

func (m *Code) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Code) DeepCopyObject() runtime.Object {
	return m
}

type CodeList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:",inline"`

	Items []Code `json:"items"`
}

func (m *CodeList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *CodeList) DeepCopyObject() runtime.Object {
	return m
}
