package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type User struct {
	TypeMeta
	ObjectMeta

	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Active   bool   `json:"active,omitempty"`
}

func (m *User) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (m *User) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}

type UserList struct {
	TypeMeta
	ListMeta
	Items []User
}

func (m *UserList) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (m *UserList) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}
