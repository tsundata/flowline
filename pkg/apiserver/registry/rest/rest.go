package rest

import (
	"context"
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"github.com/tsundata/flowline/pkg/util/uid"
	"github.com/tsundata/flowline/pkg/watch"
	"time"
)

// ResetFieldsStrategy is an optional interface that a storage object can
// implement if it wishes to provide the fields reset by its strategies.
type ResetFieldsStrategy interface {
	GetResetFields() map[string]interface{}
}

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: constant.GroupName, Version: runtime.APIVersionInternal}

// Kind takes an unqualified kind and returns a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// FillObjectMetaSystemFields populates fields that are managed by the system on ObjectMeta.
func FillObjectMetaSystemFields(o meta.Object) {
	now := time.Now()
	o.SetCreationTimestamp(&now)
	o.SetUID(uid.New())
}

// Storage is a generic interface for RESTful storage services.
// Resources which are exported to the RESTful API of apiserver need to implement this interface. It is expected
// that objects may implement any of the below interfaces.
type Storage interface {
	// New returns an empty object that can be used with Create and Update after request data has been put into it.
	// This object must be a pointer type for use with Codec.DecodeInto([]byte, runtime.Object)
	New() runtime.Object
	// NewStruct return struct
	NewStruct() interface{}

	// Destroy cleans up its resources on shutdown.
	// Destroy has to be implemented in thread-safe way and be prepared
	// for being called more than once.
	Destroy()
}

// Lister is an object that can retrieve resources that match the provided field and label criteria.
type Lister interface {
	// NewList returns an empty object that can be used with the List call.
	// This object must be a pointer type for use with Codec.DecodeInto([]byte, runtime.Object)
	NewList() runtime.Object
	// NewListStruct return struct
	NewListStruct() interface{}
	// List selects resources in the storage which match to the selector. 'options' can be nil.
	List(ctx context.Context, options *meta.ListOptions) (runtime.Object, error)
}

// Getter is an object that can retrieve a named RESTful resource.
type Getter interface {
	// Get finds a resource in the storage by name and returns it.
	// Although it can return an arbitrary error value, IsNotFound(err) is true for the
	// returned error value err when the specified resource is not found.
	Get(ctx context.Context, name string, options *meta.GetOptions) (runtime.Object, error)
}

// Deleter knows how to pass deletion options to allow delayed deletion of a
// RESTful object.
type Deleter interface {
	// Delete finds a resource in the storage and deletes it.
	// The delete attempt is validated by the deleteValidation first.
	// If options are provided, the resource will attempt to honor them or return an invalid
	// request error.
	// Although it can return an arbitrary error value, IsNotFound(err) is true for the
	// returned error value err when the specified resource is not found.
	// Delete *may* return the object that was deleted, or a status object indicating additional
	// information about deletion.
	// It also returns a boolean which is set to true if the resource was instantly
	// deleted or false if it will be deleted asynchronously.
	Delete(ctx context.Context, name string, deleteValidation ValidateObjectFunc, options *meta.DeleteOptions) (runtime.Object, bool, error)
}

// CollectionDeleter is an object that can delete a collection
// of RESTful resources.
type CollectionDeleter interface {
	// DeleteCollection selects all resources in the storage matching given 'listOptions'
	// and deletes them. The delete attempt is validated by the deleteValidation first.
	// If 'options' are provided, the resource will attempt to honor them or return an
	// invalid request error.
	// DeleteCollection may not be atomic - i.e. it may delete some objects and still
	// return an error after it. On success, returns a list of deleted objects.
	DeleteCollection(ctx context.Context, deleteValidation ValidateObjectFunc, options *meta.DeleteOptions, listOptions *meta.ListOptions) (runtime.Object, error)
}

// Creater is an object that can create an instance of a RESTful object.
type Creater interface {
	// New returns an empty object that can be used with Create after request data has been put into it.
	// This object must be a pointer type for use with Codec.DecodeInto([]byte, runtime.Object)
	New() runtime.Object

	// Create creates a new version of a resource.
	Create(ctx context.Context, obj runtime.Object, createValidation ValidateObjectFunc, options *meta.CreateOptions) (runtime.Object, error)
}

// Updater is an object that can update an instance of a RESTful object.
type Updater interface {
	// New returns an empty object that can be used with Update after request data has been put into it.
	// This object must be a pointer type for use with Codec.DecodeInto([]byte, runtime.Object)
	New() runtime.Object

	// Update finds a resource in the storage and updates it. Some implementations
	// may allow updates creates the object - they should set the created boolean
	// to true.
	Update(ctx context.Context, name string, objInfo runtime.Object, createValidation ValidateObjectFunc, updateValidation ValidateObjectUpdateFunc, forceAllowCreate bool, options *meta.UpdateOptions) (runtime.Object, bool, error)
}

// ValidateObjectFunc is a function to act on a given object. An error may be returned
// if the hook cannot be completed. A ValidateObjectFunc may NOT transform the provided
// object.
type ValidateObjectFunc func(ctx context.Context, obj runtime.Object) error

// ValidateAllObjectFunc is a "admit everything" instance of ValidateObjectFunc.
func ValidateAllObjectFunc(ctx context.Context, obj runtime.Object) error {
	return nil
}

// ValidateObjectUpdateFunc is a function to act on a given object and its predecessor.
// An error may be returned if the hook cannot be completed. An UpdateObjectFunc
// may NOT transform the provided object.
type ValidateObjectUpdateFunc func(ctx context.Context, obj, old runtime.Object) error

// ValidateAllObjectUpdateFunc is a "admit everything" instance of ValidateObjectUpdateFunc.
func ValidateAllObjectUpdateFunc(ctx context.Context, obj, old runtime.Object) error {
	return nil
}

// CreaterUpdater is a storage object that must support both create and update.
// Go prevents embedded interfaces that implement the same method.
type CreaterUpdater interface {
	Creater
	Update(ctx context.Context, name string, objInfo runtime.Object, createValidation ValidateObjectFunc, updateValidation ValidateObjectUpdateFunc, forceAllowCreate bool, options *meta.UpdateOptions) (runtime.Object, bool, error)
}

// Patcher is a storage object that supports both get and update.
type Patcher interface {
	Getter
	Updater
}

// Watcher should be implemented by all Storage objects that
// want to offer the ability to watch for changes through the watch api.
type Watcher interface {
	// Watch 'label' selects on labels; 'field' selects on the object's fields. Not all fields
	// are supported; an error should be returned if 'field' tries to select on a field that
	// isn't supported. 'resourceVersion' allows for continuing/starting a watch at a
	// particular version.
	Watch(ctx context.Context, options *meta.ListOptions) (watch.Interface, error)
}

// StandardStorage is an interface covering the common verbs. Provided for testing whether a
// resource satisfies the normal storage methods. Use Storage when passing opaque storage objects.
type StandardStorage interface {
	Getter
	Lister
	CreaterUpdater
	Deleter
	Watcher

	// Destroy cleans up its resources on shutdown.
	// Destroy has to be implemented in thread-safe way and be prepared
	// for being called more than once.
	Destroy()
}

// StorageMetadata is an optional interface that callers can implement to provide additional
// information about their Storage objects.
type StorageMetadata interface {
	// ProducesMIMETypes returns a list of the MIME types the specified HTTP verb (GET, POST, DELETE,
	// PATCH) can respond with.
	ProducesMIMETypes(verb string) []string

	// ProducesObject returns an object the specified HTTP verb respond with. It will overwrite storage object if
	// it is not nil. Only the type of the return object matters, the value will be ignored.
	ProducesObject(verb string) interface{}
}

type SubResourceStorage interface {
	StandardStorage

	Actions() []SubResourceAction

	Handle(scope interface{})
}

type SubResourceAction struct {
	Verb         string               // Verb identifying the action ("GET", "POST", "WATCH", "PROXY", etc).
	SubResource  string               // The path of the action
	Params       []*restful.Parameter // List of parameters associated with the action.
	ReadSample   interface{}
	WriteSample  interface{}
	ReturnSample interface{}
}
