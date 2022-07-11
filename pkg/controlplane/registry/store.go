package registry

import (
	"context"
	"fmt"
	"github.com/tsundata/flowline/pkg/controlplane/api/meta"
	"github.com/tsundata/flowline/pkg/controlplane/registry/generic"
	"github.com/tsundata/flowline/pkg/controlplane/registry/rest"
	"github.com/tsundata/flowline/pkg/controlplane/runtime"
	"github.com/tsundata/flowline/pkg/controlplane/runtime/schema"
	"strings"
)

type Store struct {
	// NewFunc returns a new instance of the type this registry returns for a
	// GET of a single object, e.g.:
	//
	// curl GET /apis/group/version/namespaces/my-ns/myresource/name-of-object
	NewFunc func() runtime.Object

	// NewListFunc returns a new list of the type this registry; it is the
	// type returned when the resource is listed, e.g.:
	//
	// curl GET /apis/group/version/namespaces/my-ns/myresource
	NewListFunc func() runtime.Object

	// DefaultQualifiedResource is the pluralized name of the resource.
	// This field is used if there is no request info present in the context.
	// See qualifiedResourceFromContext for details.
	DefaultQualifiedResource schema.GroupResource

	// KeyRootFunc returns the root etcd key for this resource; should not
	// include trailing "/".  This is used for operations that work on the
	// entire collection (listing and watching).
	//
	// KeyRootFunc and KeyFunc must be supplied together or not at all.
	KeyRootFunc func(ctx context.Context) string

	// KeyFunc returns the key for a specific object in the collection.
	// KeyFunc is called for Create/Update/Get/Delete. Note that 'namespace'
	// can be gotten from ctx.
	//
	// KeyFunc and KeyRootFunc must be supplied together or not at all.
	KeyFunc func(ctx context.Context, name string) (string, error)

	// ObjectNameFunc returns the name of an object or an error.
	ObjectNameFunc func(obj runtime.Object) (string, error)

	// TTLFunc returns the TTL (time to live) that objects should be persisted
	// with. The existing parameter is the current TTL or the default for this
	// operation. The update parameter indicates whether this is an operation
	// against an existing object.
	//
	// Objects that are persisted with a TTL are evicted once the TTL expires.
	TTLFunc func(obj runtime.Object, existing uint64, update bool) (uint64, error)

	// EnableGarbageCollection affects the handling of Update and Delete
	// requests. Enabling garbage collection allows finalizers to do work to
	// finalize this object before the store deletes it.
	//
	// If any store has garbage collection enabled, it must also be enabled in
	// the kube-controller-manager.
	EnableGarbageCollection bool

	// DeleteCollectionWorkers is the maximum number of workers in a single
	// DeleteCollection call. Delete requests for the items in a collection
	// are issued in parallel.
	DeleteCollectionWorkers int

	// Decorator is an optional exit hook on an object returned from the
	// underlying storage. The returned object could be an individual object
	// (e.g. Pod) or a list type (e.g. PodList). Decorator is intended for
	// integrations that are above storage and should only be used for
	// specific cases where storage of the value is not appropriate, since
	// they cannot be watched.
	Decorator func(runtime.Object)

	// CreateStrategy implements resource-specific behavior during creation.
	CreateStrategy rest.RESTCreateStrategy
	// UpdateStrategy implements resource-specific behavior during updates.
	UpdateStrategy rest.RESTUpdateStrategy
	// DeleteStrategy implements resource-specific behavior during deletion.
	DeleteStrategy rest.RESTDeleteStrategy

	// ResetFieldsStrategy provides the fields reset by the strategy that
	// should not be modified by the user.
	ResetFieldsStrategy rest.ResetFieldsStrategy

	// Storage is the interface for the underlying storage for the
	// resource. It is wrapped into a "DryRunnableStorage" that will
	// either pass-through or simply dry-run.
	Storage DryRunnableStorage
	// StorageVersioner outputs the <group/version/kind> an object will be
	// converted to before persisted in etcd, given a list of possible
	// kinds of the object.
	// If the StorageVersioner is nil, apiserver will leave the
	// storageVersionHash as empty in the discovery document.
	StorageVersioner runtime.GroupVersioner

	// DestroyFunc cleans up clients used by the underlying Storage; optional.
	// If set, DestroyFunc has to be implemented in thread-safe way and
	// be prepared for being called more than once.
	DestroyFunc func()
}

// New implements RESTStorage.New.
func (e *Store) New() runtime.Object {
	return e.NewFunc()
}

// Destroy cleans up its resources on shutdown.
func (e *Store) Destroy() {
	if e.DestroyFunc != nil {
		e.DestroyFunc()
	}
}

// NewList implements rest.Lister.
func (e *Store) NewList() runtime.Object {
	return e.NewListFunc()
}

// GetCreateStrategy implements GenericStore.
func (e *Store) GetCreateStrategy() rest.RESTCreateStrategy {
	return e.CreateStrategy
}

// GetUpdateStrategy implements GenericStore.
func (e *Store) GetUpdateStrategy() rest.RESTUpdateStrategy {
	return e.UpdateStrategy
}

// GetDeleteStrategy implements GenericStore.
func (e *Store) GetDeleteStrategy() rest.RESTDeleteStrategy {
	return e.DeleteStrategy
}

// CompleteWithOptions updates the store with the provided options and
// defaults common fields.
func (e *Store) CompleteWithOptions(options *generic.StoreOptions) error {
	if e.DefaultQualifiedResource.Empty() {
		return fmt.Errorf("store %#v must have a non-empty qualified resource", e)
	}
	if e.NewFunc == nil {
		return fmt.Errorf("store for %s must have NewFunc set", e.DefaultQualifiedResource.String())
	}
	if e.NewListFunc == nil {
		return fmt.Errorf("store for %s must have NewListFunc set", e.DefaultQualifiedResource.String())
	}
	if (e.KeyRootFunc == nil) != (e.KeyFunc == nil) {
		return fmt.Errorf("store for %s must set both KeyRootFunc and KeyFunc or neither", e.DefaultQualifiedResource.String())
	}

	if options.RESTOptions == nil {
		return fmt.Errorf("options for %s must have RESTOptions set", e.DefaultQualifiedResource.String())
	}

	opts, err := options.RESTOptions.GetRESTOptions(e.DefaultQualifiedResource)
	if err != nil {
		return err
	}

	// ResourcePrefix must come from the underlying factory
	prefix := opts.ResourcePrefix
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if prefix == "/" {
		return fmt.Errorf("store for %s has an invalid prefix %q", e.DefaultQualifiedResource.String(), opts.ResourcePrefix)
	}

	// Set the default behavior for storage key generation
	if e.KeyRootFunc == nil && e.KeyFunc == nil {

		e.KeyRootFunc = func(ctx context.Context) string {
			return prefix
		}
		e.KeyFunc = func(ctx context.Context, name string) (string, error) {
			return "", fmt.Errorf("NoNamespaceKeyFunc %s %s", prefix, name)
		}
	}

	// We adapt the store's keyFunc so that we can use it with the StorageDecorator
	// without making any assumptions about where objects are stored in etcd
	keyFunc := func(obj runtime.Object) (string, error) {
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return "", err
		}

		return e.KeyFunc(context.Background(), accessor.Name)
	}

	if e.DeleteCollectionWorkers == 0 {
		e.DeleteCollectionWorkers = opts.DeleteCollectionWorkers
	}

	e.EnableGarbageCollection = opts.EnableGarbageCollection

	if e.ObjectNameFunc == nil {
		e.ObjectNameFunc = func(obj runtime.Object) (string, error) {
			accessor, err := meta.Accessor(obj)
			if err != nil {
				return "", err
			}
			return accessor.Name, nil
		}
	}

	if e.Storage.Storage == nil {
		e.Storage.Codec = opts.StorageConfig.Codec
		var err error
		e.Storage.Storage, e.DestroyFunc, err = opts.Decorator(
			opts.StorageConfig,
			prefix,
			keyFunc,
			e.NewFunc,
			e.NewListFunc,
		)
		if err != nil {
			return err
		}
		e.StorageVersioner = opts.StorageConfig.EncodeVersioner
	}

	return nil
}
