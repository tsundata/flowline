package storage

import (
	"context"
	"github.com/tsundata/flowline/pkg/runtime"
)

// Interface offers a common interface for object marshaling/unmarshaling operations and
// hides all the storage-related operations behind it.
type Interface interface {
	// Returns Versioner associated with this interface.
	Versioner() Versioner

	// Create adds a new object at a key unless it already exists. 'ttl' is time-to-live
	// in seconds (0 means forever). If no error is returned and out is not nil, out will be
	// set to the read value from database.
	Create(ctx context.Context, key string, obj, out runtime.Object, ttl uint64) error

	// Delete removes the specified key and returns the value that existed at that spot.
	// If key didn't exist, it will return NotFound storage error.
	// If 'cachedExistingObject' is non-nil, it can be used as a suggestion about the
	// current version of the object to avoid read operation from storage to get it.
	// However, the implementations have to retry in case suggestion is stale.
	Delete(
		ctx context.Context, key string, out runtime.Object, preconditions interface{},
		validateDeletion interface{}, cachedExistingObject runtime.Object) error

	// Watch begins watching the specified key. Events are decoded into API objects,
	// and any items selected by 'p' are sent down to returned watch.Interface.
	// resourceVersion may be used to specify what version to begin watching,
	// which should be the current resourceVersion, and no longer rv+1
	// (e.g. reconnecting without missing any updates).
	// If resource version is "0", this interface will get current object at given key
	// and send it in an "ADDED" event, before watch starts.
	Watch(ctx context.Context, key string, opts ListOptions) (WatchInterface, error)

	// Get unmarshals object found at key into objPtr. On a not found error, will either
	// return a zero object of the requested type, or an error, depending on 'opts.ignoreNotFound'.
	// Treats empty responses and nil response nodes exactly like a not found error.
	// The returned contents may be delayed, but it is guaranteed that they will
	// match 'opts.ResourceVersion' according 'opts.ResourceVersionMatch'.
	Get(ctx context.Context, key string, opts GetOptions, objPtr runtime.Object) error

	// GetList unmarshalls objects found at key into a *List api object (an object
	// that satisfies runtime.IsList definition).
	// If 'opts.Recursive' is false, 'key' is used as an exact match. If `opts.Recursive'
	// is true, 'key' is used as a prefix.
	// The returned contents may be delayed, but it is guaranteed that they will
	// match 'opts.ResourceVersion' according 'opts.ResourceVersionMatch'.
	GetList(ctx context.Context, key string, opts ListOptions, listObj runtime.Object) error

	// GuaranteedUpdate keeps calling 'tryUpdate()' to update key 'key' (of type 'destination')
	// retrying the update until success if there is index conflict.
	// Note that object passed to tryUpdate may change across invocations of tryUpdate() if
	// other writers are simultaneously updating it, so tryUpdate() needs to take into account
	// the current contents of the object when deciding how the update object should look.
	// If the key doesn't exist, it will return NotFound storage error if ignoreNotFound=false
	// else `destination` will be set to the zero value of it's type.
	// If the eventual successful invocation of `tryUpdate` returns an output with the same serialized
	// contents as the input, it won't perform any update, but instead set `destination` to an object with those
	// contents.
	// If 'cachedExistingObject' is non-nil, it can be used as a suggestion about the
	// current version of the object to avoid read operation from storage to get it.
	// However, the implementations have to retry in case suggestion is stale.
	//
	// Example:
	//
	// s := /* implementation of Interface */
	// err := s.GuaranteedUpdate(
	//     "myKey", &MyType{}, true, preconditions,
	//     func(input runtime.Object, res ResponseMeta) (runtime.Object, *uint64, error) {
	//       // Before each invocation of the user defined function, "input" is reset to
	//       // current contents for "myKey" in database.
	//       curr := input.(*MyType)  // Guaranteed to succeed.
	//
	//       // Make the modification
	//       curr.Counter++
	//
	//       // Return the modified object - return an error to stop iterating. Return
	//       // a uint64 to alter the TTL on the object, or nil to keep it the same value.
	//       return cur, nil, nil
	//    }, cachedExistingObject
	// )
	GuaranteedUpdate(
		ctx context.Context, key string, destination runtime.Object, ignoreNotFound bool,
		preconditions interface{}, tryUpdate interface{}, cachedExistingObject runtime.Object) error

	// Count returns number of different entries under the key (generally being path prefix).
	Count(key string) (int64, error)
}

// WatchInterface can be implemented by anything that knows how to watch and report changes.
type WatchInterface interface {
	// Stop stops watching. Will close the channel returned by ResultChan(). Releases
	// any resources used by the watch.
	Stop()

	// ResultChan returns a chan which will receive all the events. If an error occurs
	// or Stop() is called, the implementation will close this channel and
	// release any resources used by the watch.
	ResultChan() <-chan interface{}
}

// GetOptions provides the options that may be provided for storage get operations.
type GetOptions struct {
	// IgnoreNotFound determines what is returned if the requested object is not found. If
	// true, a zero object is returned. If false, an error is returned.
	IgnoreNotFound bool
	// ResourceVersion provides a resource version constraint to apply to the get operation
	// as a "not older than" constraint: the result contains data at least as new as the provided
	// ResourceVersion. The newest available data is preferred, but any data not older than this
	// ResourceVersion may be served.
	ResourceVersion string
}

// ListOptions provides the options that may be provided for storage list operations.
type ListOptions struct {
	// ResourceVersion provides a resource version constraint to apply to the list operation
	// as a "not older than" constraint: the result contains data at least as new as the provided
	// ResourceVersion. The newest available data is preferred, but any data not older than this
	// ResourceVersion may be served.
	ResourceVersion string
	// ResourceVersionMatch provides the rule for how the resource version constraint applies. If set
	// to the default value "" the legacy resource version semantic apply.
	ResourceVersionMatch interface{}
	// Predicate provides the selection rules for the list operation.
	Predicate SelectionPredicate
	// Recursive determines whether the list or watch is defined for a single object located at the
	// given key, or for the whole set of objects with the given key as a prefix.
	Recursive bool
	// ProgressNotify determines whether storage-originated bookmark (progress notify) events should
	// be delivered to the users. The option is ignored for non-watch requests.
	ProgressNotify bool

	Limit    int64
	Continue string
}

// Versioner abstracts setting and retrieving metadata fields from database response
// onto the object ot list. It is required to maintain storage invariants - updating an
// object twice with the same data except for the ResourceVersion and SelfLink must be
// a no-op. A resourceVersion of type uint64 is a 'raw' resourceVersion,
// intended to be sent directly to or from the backend. A resourceVersion of
// type string is a 'safe' resourceVersion, intended for consumption by users.
type Versioner interface {
	// UpdateObject sets storage metadata into an API object. Returns an error if the object
	// cannot be updated correctly. May return nil if the requested object does not need metadata
	// from database.
	UpdateObject(obj interface{}, resourceVersion uint64) error
	// UpdateList sets the resource version into an API list object. Returns an error if the object
	// cannot be updated correctly. May return nil if the requested object does not need metadata from
	// database. continueValue is optional and indicates that more results are available if the client
	// passes that value to the server in a subsequent call. remainingItemCount indicates the number
	// of remaining objects if the list is partial. The remainingItemCount field is omitted during
	// serialization if it is set to nil.
	UpdateList(obj interface{}, resourceVersion uint64, continueValue string, remainingItemCount *int64) error
	// PrepareObjectForStorage should set SelfLink and ResourceVersion to the empty value. Should
	// return an error if the specified object cannot be updated.
	PrepareObjectForStorage(obj interface{}) error
	// ObjectResourceVersion returns the resource version (for persistence) of the specified object.
	// Should return an error if the specified object does not have a persistable version.
	ObjectResourceVersion(obj interface{}) (uint64, error)

	// ParseResourceVersion takes a resource version argument and
	// converts it to the storage backend. For watch we should pass to helper.Watch().
	// Because resourceVersion is an opaque value, the default watch
	// behavior for non-zero watch is to watch the next value (if you pass
	// "1", you will see updates from "2" onwards).
	ParseResourceVersion(resourceVersion string) (uint64, error)
}

// ResponseMeta contains information about the database metadata that is associated with
// an object. It abstracts the actual underlying objects to prevent coupling with concrete
// database and to improve testability.
type ResponseMeta struct {
	// TTL is the time to live of the node that contained the returned object. It may be
	// zero or negative in some cases (objects may be expired after the requested
	// expiration time due to server lag).
	TTL int64
	// The resource version of the node that contained the returned object.
	ResourceVersion uint64
}

// SelectionPredicate is used to represent the way to select objects from api storage.
type SelectionPredicate struct {
	Label               string
	Field               string
	GetAttrs            interface{}
	IndexLabels         []string
	IndexFields         []string
	Limit               int64
	Continue            string
	AllowWatchBookmarks bool
}
