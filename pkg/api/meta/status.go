package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

// Status is a return value for calls that don't return other objects.
type Status struct {
	TypeMeta `json:",inline"`
	// Standard list metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	// +optional
	ListMeta `json:"metadata,omitempty"`

	// Status of the operation.
	// One of: "Success" or "Failure".
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status string `json:"status,omitempty"`
	// A human-readable description of the status of this operation.
	// +optional
	Message string `json:"message,omitempty"`
	// A machine-readable description of why this operation is in the
	// "Failure" status. If this value is empty there
	// is no information available. A Reason clarifies an HTTP status
	// code but does not override it.
	// +optional
	Reason string `json:"reason,omitempty"`
	// Extended data associated with the reason.  Each reason may define its
	// own extended details. This field is optional and the data returned
	// is not guaranteed to conform to any schema except that defined by
	// the reason type.
	// +optional
	Details *StatusDetails `json:"details,omitempty"`
	// Suggested HTTP return code for this status, 0 if not set.
	// +optional
	Code int32 `json:"code,omitempty"`
}

func (m *Status) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Status) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}

// StatusDetails is a set of additional properties that MAY be set by the
// server to provide additional information about a response. The Reason
// field of a Status object defines what attributes will be set. Clients
// must ignore fields that do not match the defined type of each attribute,
// and should assume that any attribute may be empty, invalid, or under
// defined.
type StatusDetails struct {
	// The name attribute of the resource associated with the status StatusReason
	// (when there is a single name which can be described).
	// +optional
	Name string `json:"name,omitempty"`
	// The group attribute of the resource associated with the status StatusReason.
	// +optional
	Group string `json:"group,omitempty"`
	// The kind attribute of the resource associated with the status StatusReason.
	// On some operations may differ from the requested resource Kind.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	// +optional
	Kind string `json:"kind,omitempty"`
	// UID of the resource.
	// (when there is a single resource which can be described).
	// More info: http://kubernetes.io/docs/user-guide/identifiers#uids
	// +optional
	UID string `json:"uid,omitempty"`
	// The Causes array includes more details associated with the StatusReason
	// failure. Not all StatusReasons may provide detailed causes.
	// +optional
	Causes []string `json:"causes,omitempty"`
	// If specified, the time in seconds before the operation should be retried. Some errors may indicate
	// the client must take an alternate action - for those errors this field may indicate how long to wait
	// before taking the alternate action.
	// +optional
	RetryAfterSeconds int32 `json:"retryAfterSeconds,omitempty"`
}
