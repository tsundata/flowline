package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type Role struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:",inline"`

	Permissions []interface{} `json:"permissions,omitempty"`
}

func (m *Role) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Role) DeepCopyObject() runtime.Object {
	return m
}

type RoleList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:",inline"`

	Items []Role `json:"items"`
}

func (m *RoleList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *RoleList) DeepCopyObject() runtime.Object {
	return m
}
