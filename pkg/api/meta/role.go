package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type Role struct {
	TypeMeta
	ObjectMeta

	Permissions []interface{} `json:"permissions,omitempty"`
}

func (m *Role) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Role) DeepCopyObject() runtime.Object {
	return m
}

type RoleList struct {
	TypeMeta
	ListMeta
	Items []Role
}

func (m *RoleList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *RoleList) DeepCopyObject() runtime.Object {
	return m
}
