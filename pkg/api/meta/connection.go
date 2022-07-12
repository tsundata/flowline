package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type ProviderType string

const (
	ProviderAmazonS3 ProviderType = "amazon_s3"
	ProviderMySQL    ProviderType = "mysql"
	ProviderRedis    ProviderType = "redis"
)

type Connection struct {
	TypeMeta
	ObjectMeta

	Type        ProviderType `json:"type,omitempty"`
	Description string       `json:"description,omitempty"`
	Host        string       `json:"host,omitempty"`
	Schema      string       `json:"schema,omitempty"`
	Login       string       `json:"login,omitempty"`
	Password    string       `json:"password,omitempty"`
	Port        int          `json:"port,omitempty"`
	Extra       string       `json:"extra,omitempty"`
}

func (m *Connection) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (m *Connection) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}

type ConnectionList struct {
	TypeMeta
	ListMeta
	Items []Connection
}

func (m *ConnectionList) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (m *ConnectionList) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}
