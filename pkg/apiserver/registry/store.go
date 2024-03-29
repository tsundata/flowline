package registry

import (
	"context"
	"errors"
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/apiserver/storage"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/watch"
	"golang.org/x/xerrors"
	"strings"
)

// FinishFunc is a function returned by Begin hooks to complete an operation.
type FinishFunc func(ctx context.Context, success bool)

// AfterDeleteFunc is the type used for the Store.AfterDelete hook.
type AfterDeleteFunc func(obj runtime.Object, options *meta.DeleteOptions)

// BeginCreateFunc is the type used for the Store.BeginCreate hook.
type BeginCreateFunc func(ctx context.Context, obj runtime.Object, options *meta.CreateOptions) (FinishFunc, error)

// AfterCreateFunc is the type used for the Store.AfterCreate hook.
type AfterCreateFunc func(obj runtime.Object, options *meta.CreateOptions)

// BeginUpdateFunc is the type used for the Store.BeginUpdate hook.
type BeginUpdateFunc func(ctx context.Context, obj, old runtime.Object, options *meta.UpdateOptions) (FinishFunc, error)

// AfterUpdateFunc is the type used for the Store.AfterUpdate hook.
type AfterUpdateFunc func(obj runtime.Object, options *meta.UpdateOptions)

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

	// NewStructFunc return struct
	NewStructFunc func() interface{}

	// NewListStructFunc return struct
	NewListStructFunc func() interface{}

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
	KeyFunc func(ctx context.Context, uid string) (string, error)

	// ObjectUIDFunc returns the uid of an object or an error.
	ObjectUIDFunc func(obj runtime.Object) (string, error)

	// TTLFunc returns the TTL (time to live) that objects should be persisted
	// with. The existing parameter is the current TTL or the default for this
	// operation. The update parameter indicates whether this is an operation
	// against an existing object.
	//
	// Objects that are persisted with a TTL are evicted once the TTL expires.
	TTLFunc func(obj runtime.Object, existing uint64, update bool) (uint64, error)

	// PredicateFunc returns a matcher corresponding to the provided labels
	// and fields. The SelectionPredicate returned should return true if the
	// object matches the given field and label selectors.
	PredicateFunc func(label string, field string) meta.SelectionPredicate

	// EnableGarbageCollection affects the handling of Update and Delete
	// requests. Enabling garbage collection allows finalizers to do work to
	// finalize this object before the store deletes it.
	//
	// If any store has garbage collection enabled, it must also be enabled in
	// the controller-manager.
	EnableGarbageCollection bool

	// DeleteCollectionWorkers is the maximum number of workers in a single
	// DeleteCollection call. Delete requests for the items in a collection
	// are issued in parallel.
	DeleteCollectionWorkers int

	// Decorator is an optional exit hook on an object returned from the
	// underlying storage. The returned object could be an individual object
	// (e.g. Stage) or a list type (e.g. StageList). Decorator is intended for
	// integrations that are above storage and should only be used for
	// specific cases where storage of the value is not appropriate, since
	// they cannot be watched.
	Decorator func(runtime.Object)

	// CreateStrategy implements resource-specific behavior during creation.
	CreateStrategy rest.RESTCreateStrategy
	// BeginCreate is an optional hook that returns a "transaction-like"
	// commit/revert function which will be called at the end of the operation,
	// but before AfterCreate and Decorator, indicating via the argument
	// whether the operation succeeded.  If this returns an error, the function
	// is not called.  Almost nobody should use this hook.
	BeginCreate BeginCreateFunc
	// AfterCreate implements a further operation to run after a resource is
	// created and before it is decorated, optional.
	AfterCreate AfterCreateFunc

	// UpdateStrategy implements resource-specific behavior during updates.
	UpdateStrategy rest.RESTUpdateStrategy
	// BeginUpdate is an optional hook that returns a "transaction-like"
	// commit/revert function which will be called at the end of the operation,
	// but before AfterUpdate and Decorator, indicating via the argument
	// whether the operation succeeded.  If this returns an error, the function
	// is not called.  Almost nobody should use this hook.
	BeginUpdate BeginUpdateFunc
	// AfterUpdate implements a further operation to run after a resource is
	// updated and before it is decorated, optional.
	AfterUpdate AfterUpdateFunc

	// DeleteStrategy implements resource-specific behavior during deletion.
	DeleteStrategy rest.RESTDeleteStrategy
	// AfterDelete implements a further operation to run after a resource is
	// deleted and before it is decorated, optional.
	AfterDelete AfterDeleteFunc

	// ResetFieldsStrategy provides the fields reset by the strategy that
	// should not be modified by the user.
	ResetFieldsStrategy rest.ResetFieldsStrategy

	// Storage is the interface for the underlying storage for the
	// resource. It is wrapped into a "DryRunnableStorage" that will
	// either pass through or simply dry-run.
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

// New implements rest.Storage
func (e *Store) New() runtime.Object {
	return e.NewFunc()
}

// NewStruct implements rest.Storage
func (e *Store) NewStruct() interface{} {
	return e.NewStructFunc()
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

// NewListStruct implements rest.Lister.
func (e *Store) NewListStruct() interface{} {
	return e.NewListStructFunc()
}

// ProducesMIMETypes implements rest.StorageMetadata.
func (e *Store) ProducesMIMETypes(_ string) []string {
	return nil
}

// ProducesObject implements rest.StorageMetadata.
func (e *Store) ProducesObject(verb string) interface{} {
	switch verb {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "WATCH":
		return e.NewStruct()
	case "LIST", "DELETECOLLECTION":
		return e.NewListStruct()
	default:
		return nil
	}
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
func (e *Store) CompleteWithOptions(options *options.StoreOptions) error {
	if e.DefaultQualifiedResource.Empty() {
		return xerrors.Errorf("store %#v must have a non-empty qualified resource", e)
	}
	if e.NewFunc == nil {
		return xerrors.Errorf("store for %s must have NewFunc set", e.DefaultQualifiedResource.String())
	}
	if e.NewListFunc == nil {
		return xerrors.Errorf("store for %s must have NewListFunc set", e.DefaultQualifiedResource.String())
	}
	if (e.KeyRootFunc == nil) != (e.KeyFunc == nil) {
		return xerrors.Errorf("store for %s must set both KeyRootFunc and KeyFunc or neither", e.DefaultQualifiedResource.String())
	}

	if options.RESTOptions == nil {
		return xerrors.Errorf("options for %s must have RESTOptions set", e.DefaultQualifiedResource.String())
	}

	attrFunc := options.AttrFunc
	if attrFunc == nil {
		attrFunc = meta.DefaultClusterScopedAttr
	}
	if e.PredicateFunc == nil {
		e.PredicateFunc = func(label string, field string) meta.SelectionPredicate {
			return meta.SelectionPredicate{
				Label:    label,
				Field:    field,
				GetAttrs: attrFunc,
			}
		}
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
		return xerrors.Errorf("store for %s has an invalid prefix %q", e.DefaultQualifiedResource.String(), opts.ResourcePrefix)
	}

	// Set the default behavior for storage key generation
	if e.KeyRootFunc == nil && e.KeyFunc == nil {
		e.KeyRootFunc = func(ctx context.Context) string {
			return prefix
		}
		e.KeyFunc = func(ctx context.Context, name string) (string, error) {
			return NoNamespaceKeyFunc(ctx, prefix, name)
		}
	}

	// We adapt the store's keyFunc so that we can use it with the StorageDecorator
	// without making any assumptions about where objects are stored in etcd
	keyFunc := func(obj runtime.Object) (string, error) {
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return "", err
		}

		return e.KeyFunc(context.Background(), accessor.GetUID())
	}

	if e.DeleteCollectionWorkers == 0 {
		e.DeleteCollectionWorkers = opts.DeleteCollectionWorkers
	}

	e.EnableGarbageCollection = opts.EnableGarbageCollection

	if e.ObjectUIDFunc == nil {
		e.ObjectUIDFunc = func(obj runtime.Object) (string, error) {
			accessor, err := meta.Accessor(obj)
			if err != nil {
				return "", err
			}
			return accessor.GetUID(), nil
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

// finishNothing is a do-nothing FinishFunc.
func finishNothing(context.Context, bool) {}

func (e *Store) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *meta.CreateOptions) (runtime.Object, error) {
	var finishCreate FinishFunc = finishNothing

	if objectMeta, err := meta.Accessor(obj); err != nil {
		return nil, err
	} else {
		rest.FillObjectMetaSystemFields(objectMeta)
	}

	if e.BeginCreate != nil {
		fn, err := e.BeginCreate(ctx, obj, options)
		if err != nil {
			return nil, err
		}
		finishCreate = fn
		defer func() {
			finishCreate(ctx, false)
		}()
	}

	if err := rest.BeforeCreate(e.CreateStrategy, ctx, obj); err != nil {
		return nil, err
	}

	if createValidation != nil {
		if err := createValidation(ctx, obj); err != nil {
			return nil, err
		}
	}

	uid, err := e.ObjectUIDFunc(obj)
	if err != nil {
		return nil, err
	}
	key, err := e.KeyFunc(ctx, uid)
	if err != nil {
		return nil, err
	}
	ttl, err := e.calculateTTL(obj, 0, false)
	if err != nil {
		return nil, err
	}
	out := e.NewFunc()
	if err = e.Storage.Create(ctx, key, obj, out, ttl, false); err != nil {
		return nil, err
	}

	// The operation has succeeded.  Call the finish function if there is one,
	// and then make sure to defer doesn't call it again.
	fn := finishCreate
	finishCreate = finishNothing
	fn(ctx, true)

	if e.AfterCreate != nil {
		e.AfterCreate(out, options)
	}
	if e.Decorator != nil {
		e.Decorator(out)
	}
	return out, nil
}

func (e *Store) Update(ctx context.Context, name string, objInfo runtime.Object, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *meta.UpdateOptions) (runtime.Object, bool, error) {
	key, err := e.KeyFunc(ctx, name)
	if err != nil {
		return nil, false, err
	}

	var (
		creating = false
	)

	out := objInfo
	if err := updateValidation(ctx, objInfo, out); err != nil {
		return nil, false, err
	}

	existing := e.NewFunc()
	err = e.Storage.Get(ctx, key, meta.GetOptions{}, existing)
	if err != nil {
		return nil, false, err
	}
	if forceAllowCreate {
		if errors.Is(err, storage.ErrKeyNotFound) {
			creating = true

			// Init metadata as early as possible.
			if objectMeta, err := meta.Accessor(out); err != nil {
				return nil, creating, err
			} else {
				rest.FillObjectMetaSystemFields(objectMeta)
			}
			var finishCreate FinishFunc = finishNothing

			if e.BeginCreate != nil {
				fn, err := e.BeginCreate(ctx, out, newCreateOptionsFromUpdateOptions(options))
				if err != nil {
					return nil, creating, err
				}
				finishCreate = fn
				defer func() {
					finishCreate(ctx, false)
				}()
			}

			if err := rest.BeforeCreate(e.CreateStrategy, ctx, out); err != nil {
				return nil, creating, err
			}
			if createValidation != nil {
				if err := createValidation(ctx, out); err != nil {
					return nil, creating, err
				}
			}
			ttl, err := e.calculateTTL(out, 0, false)
			if err != nil {
				return nil, creating, err
			}
			createOut := e.NewFunc()
			err = e.Storage.Create(ctx, key, out, createOut, ttl, false)
			if err != nil {
				return nil, creating, err
			}

			// The operation has succeeded.  Call the finish function if there is one,
			// and then make sure to defer doesn't call it again.
			fn := finishCreate
			finishCreate = finishNothing
			fn(ctx, true)

			if e.AfterCreate != nil {
				e.AfterCreate(out, newCreateOptionsFromUpdateOptions(options))
			}

			return createOut, creating, nil
		}
	}

	var finishUpdate FinishFunc = finishNothing

	if e.BeginUpdate != nil {
		fn, err := e.BeginUpdate(ctx, out, existing, options)
		if err != nil {
			return nil, creating, err
		}
		finishUpdate = fn
		defer func() {
			finishUpdate(ctx, false)
		}()
	}

	if err := rest.BeforeUpdate(e.UpdateStrategy, ctx, out, existing); err != nil {
		return nil, creating, err
	}
	if updateValidation != nil {
		if err := updateValidation(ctx, out, existing); err != nil {
			return nil, creating, err
		}
	}
	_, err = e.calculateTTL(out, 0, true)
	if err != nil {
		return nil, creating, err
	}

	err = e.Storage.GuaranteedUpdate(ctx, key, out, true, nil, nil, false, nil)
	if err != nil {
		return nil, creating, err
	}

	// The operation has succeeded.  Call the finish function if there is one,
	// and then make sure to defer doesn't call it again.
	fn := finishUpdate
	finishUpdate = finishNothing
	fn(ctx, true)

	if e.AfterUpdate != nil {
		e.AfterUpdate(out, options)
	}

	if e.Decorator != nil {
		e.Decorator(out)
	}
	return out, creating, nil
}

// This is a helper to convert UpdateOptions to CreateOptions for the
// create-on-update path.
func newCreateOptionsFromUpdateOptions(in *meta.UpdateOptions) *meta.CreateOptions {
	co := &meta.CreateOptions{
		DryRun:          in.DryRun,
		FieldManager:    in.FieldManager,
		FieldValidation: in.FieldValidation,
	}
	co.TypeMeta.SetGroupVersionKind(meta.SchemeGroupVersion.WithKind("CreateOptions"))
	return co
}

func (e *Store) Get(ctx context.Context, name string, options *meta.GetOptions) (runtime.Object, error) {
	obj := e.NewFunc()
	key, err := e.KeyFunc(ctx, name)
	if err != nil {
		return nil, err
	}
	if err = e.Storage.Get(ctx, key, meta.GetOptions{ResourceVersion: options.ResourceVersion}, obj); err != nil {
		return nil, err
	}
	if e.Decorator != nil {
		e.Decorator(obj)
	}
	return obj, nil
}

func (e *Store) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc, options *meta.DeleteOptions) (runtime.Object, bool, error) {
	key, err := e.KeyFunc(ctx, name)
	if err != nil {
		return nil, false, err
	}
	obj := e.NewFunc()
	if err := deleteValidation(ctx, obj); err != nil {
		return nil, false, err
	}

	if err = e.Storage.Get(ctx, key, meta.GetOptions{}, obj); err != nil {
		return nil, false, xerrors.Errorf("InterpretDeleteError %s", err)
	}

	_, _, err = rest.BeforeDelete(e.DeleteStrategy, ctx, obj, options)
	if err != nil {
		return nil, false, err
	}

	out := e.NewFunc()
	if err = e.Storage.Delete(ctx, key, out, nil, nil, false, nil); err != nil {
		return nil, false, err
	}

	if e.AfterDelete != nil {
		e.AfterDelete(out, options)
	}

	return out, true, nil
}

// List returns a list of items matching labels and field according to the
// store's PredicateFunc.
func (e *Store) List(ctx context.Context, options *meta.ListOptions) (runtime.Object, error) {
	out, err := e.ListPredicate(ctx, meta.SelectionPredicate{}, options)
	if err != nil {
		return nil, err
	}
	if e.Decorator != nil {
		e.Decorator(out)
	}
	return out, nil
}

// ListPredicate returns a list of all the items matching the given
// SelectionPredicate.
func (e *Store) ListPredicate(ctx context.Context, p meta.SelectionPredicate, options *meta.ListOptions) (runtime.Object, error) {
	p.Limit = options.Limit
	p.Continue = options.Continue
	list := e.NewListFunc()
	storageOpts := meta.ListOptions{
		ResourceVersion:      options.ResourceVersion,
		ResourceVersionMatch: options.ResourceVersionMatch,
		Predicate:            p,
		Recursive:            true,
	}
	err := e.Storage.GetList(ctx, e.KeyRootFunc(ctx), storageOpts, list)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// Watch makes a matcher for the given label and field, and calls
// WatchPredicate. If possible, you should customize PredicateFunc to produce
// a matcher that matches by key. SelectionPredicate does this for you
// automatically.
func (e *Store) Watch(ctx context.Context, options *meta.ListOptions) (watch.Interface, error) {
	label := ""
	if options != nil && options.LabelSelector != "" {
		label = options.LabelSelector
	}
	field := ""
	if options != nil && options.FieldSelector != "" {
		field = options.FieldSelector
	}
	predicate := e.PredicateFunc(label, field)

	resourceVersion := ""
	if options != nil {
		resourceVersion = options.ResourceVersion
	}

	progressNotify := false
	if options != nil && options.ProgressNotify {
		progressNotify = options.ProgressNotify
	}

	return e.WatchPredicate(ctx, predicate, resourceVersion, progressNotify)
}

// WatchPredicate starts a watch for the items that matches.
func (e *Store) WatchPredicate(ctx context.Context, p meta.SelectionPredicate, resourceVersion string, progressNotify bool) (watch.Interface, error) {
	storageOpts := meta.ListOptions{ResourceVersion: resourceVersion, Predicate: p, Recursive: true, ProgressNotify: progressNotify}

	key := e.KeyRootFunc(ctx)
	if name, ok := p.MatchesSingle(); ok {
		if k, err := e.KeyFunc(ctx, name); err == nil {
			key = k
			storageOpts.Recursive = false
		}
		// if we cannot extract a key based on the current context, the
		// optimization is skipped
	}

	flog.Infof("(%s) etcd watch key, recursive %v", key, storageOpts.Recursive)
	w, err := e.Storage.Watch(ctx, key, storageOpts)
	if err != nil {
		return nil, err
	}
	if e.Decorator != nil {
		return newDecoratedWatcher(ctx, w, e.Decorator), nil
	}
	return w, nil
}

// calculateTTL is a helper for retrieving the updated TTL for an object or
// returning an error if the TTL cannot be calculated. The defaultTTL is
// changed to 1 if less than zero. Zero means no TTL, not expire immediately.
func (e *Store) calculateTTL(obj runtime.Object, defaultTTL int64, update bool) (ttl uint64, err error) {
	// etcd may return a negative TTL for a worker if the expiration has not
	// occurred due to server lag - we will ensure that the value is at least
	// set.
	if defaultTTL < 0 {
		defaultTTL = 1
	}
	ttl = uint64(defaultTTL)
	if e.TTLFunc != nil {
		ttl, err = e.TTLFunc(obj, ttl, update)
	}
	return ttl, err
}

// NoNamespaceKeyFunc is the default function for constructing storage paths
// to a resource relative to the given prefix without a namespace.
func NoNamespaceKeyFunc(_ context.Context, prefix string, uid string) (string, error) {
	if len(uid) == 0 {
		return "", xerrors.New("name parameter required")
	}
	if msgs := IsValidPathSegmentName(uid); len(msgs) != 0 {
		return "", xerrors.Errorf(fmt.Sprintf("UID parameter invalid: %q: %s", uid, strings.Join(msgs, ";")))
	}
	key := prefix + "/" + uid
	return key, nil
}

// NameMayNotBe specifies strings that cannot be used as names specified as path segments (like the REST API or etcd store)
var NameMayNotBe = []string{".", ".."}

// NameMayNotContain specifies substrings that cannot be used in names specified as path segments (like the REST API or etcd store)
var NameMayNotContain = []string{"/", "%"}

// IsValidPathSegmentName validates the name can be safely encoded as a path segment
func IsValidPathSegmentName(name string) []string {
	for _, illegalName := range NameMayNotBe {
		if name == illegalName {
			return []string{fmt.Sprintf(`may not be '%s'`, illegalName)}
		}
	}

	var errs []string
	for _, illegalContent := range NameMayNotContain {
		if strings.Contains(name, illegalContent) {
			errs = append(errs, fmt.Sprintf(`may not contain '%s'`, illegalContent))
		}
	}

	return errs
}
