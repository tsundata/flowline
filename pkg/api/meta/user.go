package meta

import (
	"github.com/golang-jwt/jwt/v4"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type User struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:",inline"`

	Password    string `json:"password,omitempty"`
	Email       string `json:"email,omitempty"`
	Active      bool   `json:"active,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
	UnreadCount int    `json:"unreadCount,omitempty"`
}

func (m *User) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *User) DeepCopyObject() runtime.Object {
	return m
}

type UserList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:",inline"`

	Items []User `json:"items"`
}

func (m *UserList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *UserList) DeepCopyObject() runtime.Object {
	return m
}

type UserSession struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:",inline"`

	UserUID  string `json:"userUID,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
}

func (m *UserSession) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *UserSession) DeepCopyObject() runtime.Object {
	return m
}

type UserClaims struct {
	*jwt.RegisteredClaims `json:",inline"`
}
