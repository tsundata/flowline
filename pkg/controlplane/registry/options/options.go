package options

import (
	"github.com/tsundata/flowline/pkg/controlplane/runtime/schema"
	"github.com/tsundata/flowline/pkg/controlplane/storage/config"
	"github.com/tsundata/flowline/pkg/controlplane/storage/decorator"
	"time"
)

// RESTOptions is set of resource-specific configuration options to generic registries.
type RESTOptions struct {
	StorageConfig *config.ConfigForResource
	Decorator     decorator.StorageDecorator

	EnableGarbageCollection bool
	DeleteCollectionWorkers int
	ResourcePrefix          string
	CountMetricPollPeriod   time.Duration
}

// GetRESTOptions RESTOptionsGetter so that RESTOptions can directly be used when available (i.e. tests)
func (opts RESTOptions) GetRESTOptions(schema.GroupResource) (RESTOptions, error) {
	return opts, nil
}

type RESTOptionsGetter interface {
	GetRESTOptions(resource schema.GroupResource) (RESTOptions, error)
}

// StoreOptions is set of configuration options used to complete generic registries.
type StoreOptions struct {
	RESTOptions RESTOptionsGetter
}
