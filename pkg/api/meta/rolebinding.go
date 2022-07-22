package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type RoleBinding struct {
	TypeMeta
	ObjectMeta

	UserUID string `json:"userUID,omitempty"`
	RoleUID string `json:"roleUID,omitempty"`
}

func (m *RoleBinding) GetObjectKind() schema.ObjectKind {
	return m
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
	return m
}

func (m *RoleBindingList) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}
