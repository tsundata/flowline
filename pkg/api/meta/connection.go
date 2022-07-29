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

	Type     ProviderType `json:"type,omitempty"`
	Describe string       `json:"describe,omitempty"`
	Host     string       `json:"host,omitempty"`
	Schema   string       `json:"schema,omitempty"`
	Login    string       `json:"login,omitempty"`
	Password string       `json:"password,omitempty"`
	Port     int          `json:"port,omitempty"`
	Extra    string       `json:"extra,omitempty"`
}

func (m *Connection) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Connection) DeepCopyObject() runtime.Object {
	return m
}

type ConnectionList struct {
	TypeMeta
	ListMeta
	Items []Connection
}

func (m *ConnectionList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *ConnectionList) DeepCopyObject() runtime.Object {
	return m
}
