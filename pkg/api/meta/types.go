package meta

// ListOptions is the query options to a standard REST list call.
type ListOptions struct {
	TypeMeta `json:",inline"`

	// A selector to restrict the list of returned objects by their labels.
	// Defaults to everything.
	// +optional
	LabelSelector string `json:"labelSelector,omitempty" protobuf:"bytes,1,opt,name=labelSelector"`
	// A selector to restrict the list of returned objects by their fields.
	// Defaults to everything.
	// +optional
	FieldSelector string `json:"fieldSelector,omitempty" protobuf:"bytes,2,opt,name=fieldSelector"`

	// +k8s:deprecated=includeUninitialized,protobuf=6

	// Watch for changes to the described resources and return them as a stream of
	// add, update, and remove notifications. Specify resourceVersion.
	// +optional
	Watch bool `json:"watch,omitempty" protobuf:"varint,3,opt,name=watch"`
	// allowWatchBookmarks requests watch events with type "BOOKMARK".
	// Servers that do not implement bookmarks may ignore this flag and
	// bookmarks are sent at the server's discretion. Clients should not
	// assume bookmarks are returned at any specific interval, nor may they
	// assume the server will send any BOOKMARK event during a session.
	// If this is not a watch, this field is ignored.
	// +optional
	AllowWatchBookmarks bool `json:"allowWatchBookmarks,omitempty" protobuf:"varint,9,opt,name=allowWatchBookmarks"`

	// resourceVersion sets a constraint on what resource versions a request may be served from.
	// See https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-versions for
	// details.
	//
	// Defaults to unset
	// +optional
	ResourceVersion string `json:"resourceVersion,omitempty" protobuf:"bytes,4,opt,name=resourceVersion"`

	// resourceVersionMatch determines how resourceVersion is applied to list calls.
	// It is highly recommended that resourceVersionMatch be set for list calls where
	// resourceVersion is set
	// See https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-versions for
	// details.
	//
	// Defaults to unset
	// +optional
	ResourceVersionMatch string `json:"resourceVersionMatch,omitempty" protobuf:"bytes,10,opt,name=resourceVersionMatch,casttype=ResourceVersionMatch"`
	// Timeout for the list/watch call.
	// This limits the duration of the call, regardless of any activity or inactivity.
	// +optional
	TimeoutSeconds *int64 `json:"timeoutSeconds,omitempty" protobuf:"varint,5,opt,name=timeoutSeconds"`

	// limit is a maximum number of responses to return for a list call. If more items exist, the
	// server will set the `continue` field on the list metadata to a value that can be used with the
	// same initial query to retrieve the next set of results. Setting a limit may return fewer than
	// the requested amount of items (up to zero items) in the event all requested objects are
	// filtered out and clients should only use the presence of the continue field to determine whether
	// more results are available. Servers may choose not to support the limit argument and will return
	// all of the available results. If limit is specified and the continue field is empty, clients may
	// assume that no more results are available. This field is not supported if watch is true.
	//
	// The server guarantees that the objects returned when using continue will be identical to issuing
	// a single list call without a limit - that is, no objects created, modified, or deleted after the
	// first request is issued will be included in any subsequent continued requests. This is sometimes
	// referred to as a consistent snapshot, and ensures that a client that is using limit to receive
	// smaller chunks of a very large result can ensure they see all possible objects. If objects are
	// updated during a chunked list the version of the object that was present at the time the first list
	// result was calculated is returned.
	Limit int64 `json:"limit,omitempty" protobuf:"varint,7,opt,name=limit"`
	// The continue option should be set when retrieving more results from the server. Since this value is
	// server defined, clients may only use the continue value from a previous query result with identical
	// query parameters (except for the value of continue) and the server may reject a continue value it
	// does not recognize. If the specified continue value is no longer valid whether due to expiration
	// (generally five to fifteen minutes) or a configuration change on the server, the server will
	// respond with a 410 ResourceExpired error together with a continue token. If the client needs a
	// consistent list, it must restart their list without the continue field. Otherwise, the client may
	// send another list request with the token received with the 410 error, the server will respond with
	// a list starting from the next key, but from the latest snapshot, which is inconsistent from the
	// previous list results - objects that are created, modified, or deleted after the first list request
	// will be included in the response, as long as their keys are after the "next key".
	//
	// This field is not supported when watch is true. Clients may start a watch from the last
	// resourceVersion value returned by the server and not miss any modifications.
	Continue string `json:"continue,omitempty" protobuf:"bytes,8,opt,name=continue"`
}
