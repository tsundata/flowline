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

	Roles []string `json:"roles,omitempty"`
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

type Dashboard struct {
	WorkflowAmount int64 `json:"workflowAmount"`
	CodeAmount     int64 `json:"codeAmount"`
	VariableAmount int64 `json:"variableAmount"`
	WorkerAmount   int64 `json:"workerAmount"`

	Data []DashboardData `json:"data,omitempty"`
}

type DashboardData struct {
	Date     string `json:"date"`
	Schedule int    `json:"schedule"`
}

type Policy struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:",inline"`

	Key   string `json:"key"`
	PType string `json:"ptype"`
	V0    string `json:"v0"`
	V1    string `json:"v1"`
	V2    string `json:"v2"`
	V3    string `json:"v3"`
	V4    string `json:"v4"`
	V5    string `json:"v5"`
}

func (m *Policy) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Policy) DeepCopyObject() runtime.Object {
	return m
}

type PolicyList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:",inline"`

	Items []Policy `json:"items"`
}

func (m *PolicyList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *PolicyList) DeepCopyObject() runtime.Object {
	return m
}
