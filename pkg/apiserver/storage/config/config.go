package config

import (
	"github.com/tsundata/flowline/pkg/apiserver/storage/etcd"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"time"
)

const (
	DefaultCompactInterval      = 5 * time.Minute
	DefaultDBMetricPollInterval = 30 * time.Second
	DefaultHealthcheckTimeout   = 2 * time.Second
)

// TransportConfig holds all connection related info,  i.e. equal TransportConfig means equal servers we talk to.
type TransportConfig struct {
	// ServerList is the list of storage servers to connect with.
	ServerList []string
	// TLS credentials
	KeyFile       string
	CertFile      string
	TrustedCAFile string
}

// Config is configuration for creating a storage backend.
type Config struct {
	// Type defines the type of storage backend. Default ("") is "etcd3".
	Type string
	// Prefix is the prefix to all keys passed to storage.Interface methods.
	Prefix string
	// Transport holds all connection related info, i.e. equal TransportConfig means equal servers we talk to.
	Transport TransportConfig
	// Paging indicates whether the server implementation should allow paging (if it is
	// supported). This is generally configured by feature gating, or by a specific
	// resource type not wishing to allow paging, and is not intended for end users to
	// set.
	Paging bool

	Codec runtime.Codec
	// EncodeVersioner is the same groupVersioner used to build the
	// storage encoder. Given a list of kinds the input object might belong
	// to, the EncodeVersioner outputs the gvk the object will be
	// converted to before persisted in etcd.
	EncodeVersioner runtime.GroupVersioner

	// CompactionInterval is an interval of requesting compaction from apiserver.
	// If the value is 0, no compaction will be issued.
	CompactionInterval time.Duration
	// CountMetricPollPeriod specifies how often should count metric be updated
	CountMetricPollPeriod time.Duration
	// DBMetricPollInterval specifies how often should storage backend metric be updated.
	DBMetricPollInterval time.Duration
	// HealthcheckTimeout specifies the timeout used when checking health
	HealthcheckTimeout time.Duration

	LeaseManagerConfig etcd.LeaseManagerConfig
}

// ConfigForResource is a Config specialized to a particular `schema.GroupResource`
type ConfigForResource struct {
	// Config is the resource-independent configuration
	Config

	// GroupResource is the relevant one
	GroupResource schema.GroupResource
}

// ForResource specializes to the given resource
func (config *Config) ForResource(resource schema.GroupResource) *ConfigForResource {
	return &ConfigForResource{
		Config:        *config,
		GroupResource: resource,
	}
}

func NewDefaultConfig(prefix string, codec runtime.Codec) *Config {
	return &Config{
		Paging:               true,
		Prefix:               prefix,
		Codec:                codec,
		CompactionInterval:   DefaultCompactInterval,
		DBMetricPollInterval: DefaultDBMetricPollInterval,
		HealthcheckTimeout:   DefaultHealthcheckTimeout,
		LeaseManagerConfig:   etcd.NewDefaultLeaseManagerConfig(),
	}
}
