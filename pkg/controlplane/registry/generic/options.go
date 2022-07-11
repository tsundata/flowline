package generic

import (
	"github.com/tsundata/flowline/pkg/controlplane/runtime"
	"github.com/tsundata/flowline/pkg/controlplane/runtime/schema"
	"github.com/tsundata/flowline/pkg/controlplane/storage"
	"time"
)

// RESTOptions is set of resource-specific configuration options to generic registries.
type RESTOptions struct {
	StorageConfig *storage.ConfigForResource
	Decorator     StorageDecorator

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

// StorageDecorator is a function signature for producing a storage.Interface
// and an associated DestroyFunc from given parameters.
type StorageDecorator func(
	config *storage.ConfigForResource,
	resourcePrefix string,
	keyFunc func(obj runtime.Object) (string, error),
	newFunc func() runtime.Object,
	newListFunc func() runtime.Object) (storage.Interface, DestroyFunc, error)

type DestroyFunc func()
