package decorator

import (
	"github.com/tsundata/flowline/pkg/apiserver/storage"
	"github.com/tsundata/flowline/pkg/apiserver/storage/config"
	"github.com/tsundata/flowline/pkg/runtime"
)

// StorageDecorator is a function signature for producing a storage.Interface
// and an associated DestroyFunc from given parameters.
type StorageDecorator func(
	config *config.ConfigForResource,
	resourcePrefix string,
	keyFunc func(obj runtime.Object) (string, error),
	newFunc func() runtime.Object,
	newListFunc func() runtime.Object) (storage.Interface, DestroyFunc, error)

type DestroyFunc func()
