package decorator

import (
	"github.com/tsundata/flowline/pkg/controlplane/runtime"
	"github.com/tsundata/flowline/pkg/controlplane/storage"
	"github.com/tsundata/flowline/pkg/controlplane/storage/config"
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
