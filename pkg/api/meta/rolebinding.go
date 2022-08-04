package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type RoleBinding struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:",inline"`

	UserUID string `json:"userUID,omitempty"`
	RoleUID string `json:"roleUID,omitempty"`
}

func (m *RoleBinding) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *RoleBinding) DeepCopyObject() runtime.Object {
	return m
}

type RoleBindingList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:",inline"`
	
	Items []RoleBinding `json:"items"`
}

func (m *RoleBindingList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *RoleBindingList) DeepCopyObject() runtime.Object {
	return m
}
