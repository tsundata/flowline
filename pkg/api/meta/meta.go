package meta

import (
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"time"
)

// PatchType Similarly to above, these are constants to support HTTP PATCH utilized by
// both the client and server that didn't make sense for a whole package to be
// dedicated to.
type PatchType string

const (
	JSONPatchType           PatchType = "application/json-patch+json"
	MergePatchType          PatchType = "application/merge-patch+json"
	StrategicMergePatchType PatchType = "application/strategic-merge-patch+json"
	ApplyPatchType          PatchType = "application/apply-patch+yaml"
)

// TypeMeta describes an individual object in an API response or request
// with strings representing the type of the object and its API schema version.
// Structures that are versioned or persisted should inline TypeMeta.
type TypeMeta struct {
	// Kind is a string value representing the REST resource this object represents.
	// Servers may infer this from the endpoint the client submits requests to.
	// Cannot be updated.
	// In CamelCase.
	Kind string `json:"kind,omitempty"`

	// APIVersion defines the versioned schema of this representation of an object.
	// Servers should convert recognized schemas to the latest internal value, and
	// may reject unrecognized values.
	APIVersion string `json:"apiVersion,omitempty"`
}

func (t *TypeMeta) SetGroupVersionKind(kind schema.GroupVersionKind) {
	t.Kind = kind.Kind
}

func (t *TypeMeta) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   constant.GroupName,
		Version: t.APIVersion,
		Kind:    t.Kind,
	}
}

// ObjectMeta is metadata that all persisted resources must have, which includes all objects
// users must create.
type ObjectMeta struct {
	Name string `json:"name,omitempty"`
	UID  string `json:"uid,omitempty"`

	ResourceVersion string `json:"resourceVersion,omitempty"`
	Generation      int64  `json:"generation,omitempty"`

	CreationTimestamp          *time.Time `json:"creationTimestamp,omitempty"`
	DeletionTimestamp          *time.Time `json:"deletionTimestamp,omitempty"`
	DeletionGracePeriodSeconds *int64     `json:"deletionGracePeriodSeconds,omitempty"`

	Labels map[string]string `json:"labels,omitempty"`

	Finalizers []string `json:"finalizers,omitempty"`
}

func (o *ObjectMeta) GetObjectMeta() Object {
	return o
}

func (o *ObjectMeta) GetName() string {
	return o.Name
}

func (o *ObjectMeta) SetName(name string) {
	o.Name = name
}

func (o *ObjectMeta) GetUID() string {
	return o.UID
}

func (o *ObjectMeta) SetUID(uid string) {
	o.UID = uid
}

func (o *ObjectMeta) GetResourceVersion() string {
	return o.ResourceVersion
}

func (o *ObjectMeta) SetResourceVersion(version string) {
	o.ResourceVersion = version
}

func (o *ObjectMeta) GetGeneration() int64 {
	return o.Generation
}

func (o *ObjectMeta) SetGeneration(generation int64) {
	o.Generation = generation
}

func (o *ObjectMeta) GetCreationTimestamp() *time.Time {
	return o.CreationTimestamp
}

func (o *ObjectMeta) SetCreationTimestamp(timestamp *time.Time) {
	o.CreationTimestamp = timestamp
}

func (o *ObjectMeta) GetDeletionTimestamp() *time.Time {
	return o.DeletionTimestamp
}

func (o *ObjectMeta) SetDeletionTimestamp(timestamp *time.Time) {
	o.DeletionTimestamp = timestamp
}

func (o *ObjectMeta) GetDeletionGracePeriodSeconds() *int64 {
	return o.DeletionGracePeriodSeconds
}

func (o *ObjectMeta) SetDeletionGracePeriodSeconds(i *int64) {
	o.DeletionGracePeriodSeconds = i
}

func (o *ObjectMeta) GetLabels() map[string]string {
	return o.Labels
}

func (o *ObjectMeta) SetLabels(labels map[string]string) {
	o.Labels = labels
}

// ListMeta describes metadata that synthetic resources must have, including lists and
// various status objects. A resource may have only one of {ObjectMeta, ListMeta}.
type ListMeta struct {
	// String that identifies the server's internal version of this object that
	// can be used by clients to determine when objects have changed.
	// Value must be treated as opaque by clients and passed unmodified back to the server.
	// Populated by the system.
	// Read-only.
	ResourceVersion string `json:"resourceVersion,omitempty"`

	// continue may be set if the user set a limit on the number of items returned, and indicates that
	// the server has more data available. The value is opaque and may be used to issue another request
	// to the endpoint that served this list to retrieve the next set of available objects. Continuing a
	// consistent list may not be possible if the server configuration has changed or more than a few
	// minutes have passed. The resourceVersion field returned when using this continue value will be
	// identical to the value in the first response, unless you have received this token from an error
	// message.
	Continue string `json:"continue,omitempty"`

	// remainingItemCount is the number of subsequent items in the list which are not included in this
	// list response. If the list request contained label or field selectors, then the number of
	// remaining items is unknown and the field will be left unset and omitted during serialization.
	// If the list is complete (either because it is not chunking or because this is the last chunk),
	// then there are no more remaining items and this field will be left unset and omitted during
	// serialization.
	// Servers older than v1.15 do not set this field.
	// The intended use of the remainingItemCount is *estimating* the size of a collection. Clients
	// should not rely on the remainingItemCount to be set or to be exact.
	// +optional
	RemainingItemCount *int64 `json:"remainingItemCount,omitempty"`
}

func (l *ListMeta) GetListMeta() List {
	return l
}

func (l *ListMeta) GetResourceVersion() string {
	return l.ResourceVersion
}

func (l *ListMeta) SetResourceVersion(version string) {
	l.ResourceVersion = version
}

func (l *ListMeta) GetContinue() string {
	return l.Continue
}

func (l *ListMeta) SetContinue(c string) {
	l.Continue = c
}

func (l *ListMeta) GetRemainingItemCount() *int64 {
	return l.RemainingItemCount
}

func (l *ListMeta) SetRemainingItemCount(c *int64) {
	l.RemainingItemCount = c
}

// HasObjectMetaSystemFieldValues returns true if fields that are managed by the system on ObjectMeta have values.
func HasObjectMetaSystemFieldValues(meta Object) bool {
	return !meta.GetCreationTimestamp().IsZero() ||
		len(meta.GetUID()) != 0
}

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: constant.GroupName, Version: "v1"}
