package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type RoleBinding struct {
	TypeMeta
	ObjectMeta

	UserUID string `json:"user_uid,omitempty"`
	RoleUID string `json:"role_uid,omitempty"`
}

func (m *RoleBinding) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (m *RoleBinding) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}

type RoleBindingList struct {
	TypeMeta
	ListMeta
	Items []RoleBinding
}

func (m *RoleBindingList) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (m *RoleBindingList) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}
