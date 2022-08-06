package meta

import "github.com/tsundata/flowline/pkg/runtime"

// GetOptions is the standard query options to the standard REST get call.
type GetOptions struct {
	TypeMeta `json:",inline"`
	// resourceVersion sets a constraint on what resource versions a request may be served from.
	//
	// Defaults to unset
	ResourceVersion string `json:"resourceVersion,omitempty"`

	// IgnoreNotFound determines what is returned if the requested object is not found. If
	// true, a zero object is returned. If false, an error is returned.
	IgnoreNotFound bool
}

// CreateOptions may be provided when creating an API object.
type CreateOptions struct {
	TypeMeta `json:",inline"`

	// When present, indicates that modifications should not be
	// persisted. An invalid or unrecognized dryRun directive will
	// result in an error response and no further processing of the
	// request. Valid values are:
	// - All: all dry run stages will be processed
	DryRun []string `json:"dryRun,omitempty"`

	// fieldManager is a name associated with the actor or entity
	// that is making these changes. The value must be less than or
	// 128 characters long, and only contain printable characters,
	// as defined by https://golang.org/pkg/unicode/#IsPrint.
	FieldManager string `json:"fieldManager,omitempty"`

	// fieldValidation instructs the server on how to handle
	// objects in the request (POST/PUT/PATCH) containing unknown
	// or duplicate fields, provided that the `ServerSideFieldValidation`
	// feature gate is also enabled. Valid values are:
	// - Ignore: This will ignore any unknown fields that are silently
	// dropped from the object, and will ignore all but the last duplicate
	// field that the decoder encounters. This is the default behavior
	// prior to v1.23 and is the default behavior when the
	// `ServerSideFieldValidation` feature gate is disabled.
	// - Warn: This will send a warning via the standard warning response
	// header for each unknown field that is dropped from the object, and
	// for each duplicate field that is encountered. The request will
	// still succeed if there are no other errors, and will only persist
	// the last of any duplicate fields. This is the default when the
	// `ServerSideFieldValidation` feature gate is enabled.
	// - Strict: This will fail the request with a BadRequest error if
	// any unknown fields would be dropped from the object, or if any
	// duplicate fields are present. The error returned from the server
	// will contain all unknown and duplicate fields encountered.
	FieldValidation string `json:"fieldValidation,omitempty"`
}

// PatchOptions may be provided when patching an API object.
// PatchOptions is meant to be a superset of UpdateOptions.
type PatchOptions struct {
	TypeMeta `json:",inline"`

	// When present, indicates that modifications should not be
	// persisted. An invalid or unrecognized dryRun directive will
	// result in an error response and no further processing of the
	// request. Valid values are:
	// - All: all dry run stages will be processed
	DryRun []string `json:"dryRun,omitempty"`

	// Force is going to "force" Apply requests. It means user will
	// re-acquire conflicting fields owned by other people. Force
	// flag must be unset for non-apply patch requests.
	Force *bool `json:"force,omitempty"`

	// fieldManager is a name associated with the actor or entity
	// that is making these changes. The value must be less than or
	// 128 characters long, and only contain printable characters,
	// as defined by https://golang.org/pkg/unicode/#IsPrint. This
	// field is required for apply requests
	// (application/apply-patch) but optional for non-apply patch
	// types (JsonPatch, MergePatch, StrategicMergePatch).
	// +optional
	FieldManager string `json:"fieldManager,omitempty"`

	// fieldValidation instructs the server on how to handle
	// objects in the request (POST/PUT/PATCH) containing unknown
	// or duplicate fields, provided that the `ServerSideFieldValidation`
	// feature gate is also enabled. Valid values are:
	// - Ignore: This will ignore any unknown fields that are silently
	// dropped from the object, and will ignore all but the last duplicate
	// field that the decoder encounters. This is the default behavior
	// prior to v1.23 and is the default behavior when the
	// `ServerSideFieldValidation` feature gate is disabled.
	// - Warn: This will send a warning via the standard warning response
	// header for each unknown field that is dropped from the object, and
	// for each duplicate field that is encountered. The request will
	// still succeed if there are no other errors, and will only persist
	// the last of any duplicate fields. This is the default when the
	// `ServerSideFieldValidation` feature gate is enabled.
	// - Strict: This will fail the request with a BadRequest error if
	// any unknown fields would be dropped from the object, or if any
	// duplicate fields are present. The error returned from the server
	// will contain all unknown and duplicate fields encountered.
	// +optional
	FieldValidation string `json:"fieldValidation,omitempty"`
}

// ListOptions is the query options to a standard REST list call.
type ListOptions struct {
	TypeMeta `json:",inline"`

	// A selector to restrict the list of returned objects by their labels.
	// Defaults to everything.
	LabelSelector string `json:"labelSelector,omitempty"`
	// A selector to restrict the list of returned objects by their fields.
	// Defaults to everything.
	FieldSelector string `json:"fieldSelector,omitempty"`

	// Watch for changes to the described resources and return them as a stream of
	// add, update, and remove notifications. Specify resourceVersion.
	// +optional
	Watch bool `json:"watch,omitempty"`
	// allowWatchBookmarks requests watch events with type "BOOKMARK".
	// Servers that do not implement bookmarks may ignore this flag and
	// bookmarks are sent at the server's discretion. Clients should not
	// assume bookmarks are returned at any specific interval, nor may they
	// assume the server will send any BOOKMARK event during a session.
	// If this is not a watch, this field is ignored.
	AllowWatchBookmarks bool `json:"allowWatchBookmarks,omitempty"`

	// resourceVersion sets a constraint on what resource versions a request may be served from.
	//
	// Defaults to unset
	ResourceVersion string `json:"resourceVersion,omitempty"`

	// resourceVersionMatch determines how resourceVersion is applied to list calls.
	// It is highly recommended that resourceVersionMatch be set for list calls where
	// resourceVersion is set
	//
	// Defaults to unset
	ResourceVersionMatch string `json:"resourceVersionMatch,omitempty"`
	// Timeout for the list/watch call.
	// This limits the duration of the call, regardless of any activity or inactivity.
	TimeoutSeconds *int64 `json:"timeoutSeconds,omitempty"`

	// limit is a maximum number of responses to return for a list call. If more items exist, the
	// server will set the `continue` field on the list metadata to a value that can be used with the
	// same initial query to retrieve the next set of results. Setting a limit may return fewer than
	// the requested amount of items (up to zero items) in the event all requested objects are
	// filtered out and clients should only use the presence of the continue field to determine whether
	// more results are available. Servers may choose not to support the limit argument and will return
	// all the available results. If limit is specified and the continue field is empty, clients may
	// assume that no more results are available. This field is not supported if watch is true.
	//
	// The server guarantees that the objects returned when using continue will be identical to issuing
	// a single list call without a limit - that is, no objects created, modified, or deleted after the
	// first request is issued will be included in any subsequent continued requests. This is sometimes
	// referred to as a consistent snapshot, and ensures that a client that is using limit to receive
	// smaller chunks of a very large result can ensure they see all possible objects. If objects are
	// updated during a chunked list the version of the object that was present at the time the first list
	// result was calculated is returned.
	Limit int64 `json:"limit,omitempty"`
	// The continue option should be set when retrieving more results from the server. Since this value is
	// server defined, clients may only use to continue value from a previous query result with identical
	// query parameters (except for the value of continue) and the server may reject a continue value it
	// does not recognize. If the specified continue value is no longer valid whether due to expiration
	// (generally five to fifteen minutes) or a configuration change on the server, the server will
	// respond with a 410 ResourceExpired error together with a continued token. If the client needs a
	// consistent list, it must restart their list without the continue field. Otherwise, the client may
	// send another list request with the token received with the 410 error, the server will respond with
	// a list starting from the next key, but from the latest snapshot, which is inconsistent from the
	// previous list results - objects that are created, modified, or deleted after the first list request
	// will be included in the response, as long as their keys are after the "next key".
	//
	// This field is not supported when watch is true. Clients may start a watch from the last
	// resourceVersion value returned by the server and not miss any modifications.
	Continue string `json:"continue,omitempty"`

	// Predicate provides the selection rules for the list operation.
	Predicate SelectionPredicate
	// Recursive determines whether the list or watch is defined for a single object located at the
	// given key, or for the whole set of objects with the given key as a prefix.
	Recursive bool `json:"recursive,omitempty"`
	// ProgressNotify determines whether storage-originated bookmark (progress notify) events should
	// be delivered to the users. The option is ignored for non-watch requests.
	ProgressNotify bool `json:"progressNotify,omitempty"`
}

// AttrFunc returns label and field sets and the uninitialized flag for List or Watch to match.
// In any failure to parse given object, it returns error.
type AttrFunc func(obj runtime.Object) (map[string]string, map[string]string, error)

func DefaultClusterScopedAttr(obj runtime.Object) (map[string]string, map[string]string, error) {
	metadata, err := Accessor(obj)
	if err != nil {
		return nil, nil, err
	}
	fieldSet := map[string]string{
		"metadata.name": metadata.GetName(),
	}

	return metadata.GetLabels(), fieldSet, nil
}

// SelectionPredicate is used to represent the way to select objects from api storage.
type SelectionPredicate struct {
	Label               string
	Field               string
	GetAttrs            AttrFunc
	IndexLabels         []string
	IndexFields         []string
	Limit               int64
	Continue            string
	AllowWatchBookmarks bool
}

// Matches returns true if the given object's labels and fields (as
// returned by s.GetAttrs) match s.Label and s.Field. An error is
// returned if s.GetAttrs fails.
func (s *SelectionPredicate) Matches(obj runtime.Object) (bool, error) {
	if s.Empty() {
		return true, nil
	}
	labels, fields, err := s.GetAttrs(obj)
	if err != nil {
		return false, err
	}
	matched := false
	for _, label := range labels {
		if s.Label == label {
			matched = true
		}
	}
	if matched && s.Field != "" {
		matched2 := false
		for _, field := range fields {
			if s.Field == field {
				matched2 = true
			}
		}
		matched = matched && matched2
	}
	return matched, nil
}

// Empty returns true if the predicate performs no filtering.
func (s *SelectionPredicate) Empty() bool {
	return s.Label == "" && s.Field == ""
}

// MatchesSingle will return (name, true) if and only if s.Field matches on the object's
// name.
func (s *SelectionPredicate) MatchesSingle() (string, bool) {
	if len(s.Field) == 0 {
		return "", false
	}
	// field --> metadata.name
	return s.Field, true
}

// Everything accepts all objects.
var Everything = SelectionPredicate{
	Label: "",
	Field: "",
}

// UpdateOptions may be provided when updating an API object.
// All fields in UpdateOptions should also be present in PatchOptions.
type UpdateOptions struct {
	TypeMeta `json:",inline"`

	// When present, indicates that modifications should not be
	// persisted. An invalid or unrecognized dryRun directive will
	// result in an error response and no further processing of the
	// request. Valid values are:
	// - All: all dry run stages will be processed
	DryRun []string `json:"dryRun,omitempty"`

	// fieldManager is a name associated with the actor or entity
	// that is making these changes. The value must be less than or
	// 128 characters long, and only contain printable characters,
	// as defined by https://golang.org/pkg/unicode/#IsPrint.
	FieldManager string `json:"fieldManager,omitempty"`

	// fieldValidation instructs the server on how to handle
	// objects in the request (POST/PUT/PATCH) containing unknown
	// or duplicate fields, provided that the `ServerSideFieldValidation`
	// feature gate is also enabled. Valid values are:
	// - Ignore: This will ignore any unknown fields that are silently
	// dropped from the object, and will ignore all but the last duplicate
	// field that the decoder encounters. This is the default behavior
	// prior to v1.23 and is the default behavior when the
	// `ServerSideFieldValidation` feature gate is disabled.
	// - Warn: This will send a warning via the standard warning response
	// header for each unknown field that is dropped from the object, and
	// for each duplicate field that is encountered. The request will
	// still succeed if there are no other errors, and will only persist
	// the last of any duplicate fields. This is the default when the
	// `ServerSideFieldValidation` feature gate is enabled.
	// - Strict: This will fail the request with a BadRequest error if
	// any unknown fields would be dropped from the object, or if any
	// duplicate fields are present. The error returned from the server
	// will contain all unknown and duplicate fields encountered.
	FieldValidation string `json:"fieldValidation,omitempty"`
}

// DeleteOptions may be provided when deleting an API object.
type DeleteOptions struct {
	TypeMeta `json:",inline"`

	// The duration in seconds before the object should be deleted. Value must be non-negative integer.
	// The value zero indicates delete immediately. If this value is nil, the default grace period for the
	// specified type will be used.
	// Defaults to a per object value if not specified. zero means delete immediately.
	GracePeriodSeconds *int64 `json:"gracePeriodSeconds,omitempty"`

	// Must be fulfilled before a deletion is carried out. If not possible, a 409 Conflict status will be
	// returned.
	Preconditions *Preconditions `json:"preconditions,omitempty"`

	// Deprecated: please use the PropagationPolicy, this field will be deprecated in 1.7.
	// Should the dependent objects be orphaned. If true/false, the "orphan"
	// finalizer will be added to/removed from the object's finalizers list.
	// Either this field or PropagationPolicy may be set, but not both.
	OrphanDependents *bool `json:"orphanDependents,omitempty"`

	// Whether and how garbage collection will be performed.
	// Either this field or OrphanDependents may be set, but not both.
	// The default policy is decided by the existing finalizer set in the
	// metadata.finalizers and the resource-specific default policy.
	// Acceptable values are: 'Orphan' - orphan the dependents; 'Background' -
	// allow the garbage collector to delete the dependents in the background;
	// 'Foreground' - a cascading policy that deletes all dependents in the
	// foreground.
	PropagationPolicy *string `json:"propagationPolicy,omitempty"`

	// When present, indicates that modifications should not be
	// persisted. An invalid or unrecognized dryRun directive will
	// result in an error response and no further processing of the
	// request. Valid values are:
	// - All: all dry run stages will be processed
	DryRun []string `json:"dryRun,omitempty"`
}

// Preconditions must be fulfilled before an operation (update, delete, etc.) is carried out.
type Preconditions struct {
	// Specifies the target UID.
	UID *string `json:"uid,omitempty"`
	// Specifies the target ResourceVersion
	ResourceVersion *string `json:"resourceVersion,omitempty"`
}
