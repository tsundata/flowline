package rest

import (
	"context"
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"github.com/tsundata/flowline/pkg/util/uid"
	"time"
)

// ResetFieldsStrategy is an optional interface that a storage object can
// implement if it wishes to provide the fields reset by its strategies.
type ResetFieldsStrategy interface {
	GetResetFields() map[string]interface{}
}

// GroupName is the group name use in this package
const GroupName = "apps"

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: runtime.APIVersionInternal}

// Kind takes an unqualified kind and returns a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// ValidateObjectFunc is a function to act on a given object. An error may be returned
// if the hook cannot be completed. A ValidateObjectFunc may NOT transform the provided
// object.
type ValidateObjectFunc func(ctx context.Context, obj runtime.Object) error

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

	// Destroy cleans up its resources on shutdown.
	// Destroy has to be implemented in thread-safe way and be prepared
	// for being called more than once.
	Destroy()

	//Handler route handler
	Handler
}

//Handler route handler
type Handler interface {
	GetHandler(req *restful.Request, resp *restful.Response)
	PostHandler(req *restful.Request, resp *restful.Response)
	PutHandler(req *restful.Request, resp *restful.Response)
	DeleteHandler(req *restful.Request, resp *restful.Response)
}
