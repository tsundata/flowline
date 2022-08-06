package meta

import (
	"github.com/barkimedes/go-deepcopy"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"time"
)

// Valid values for event types (new types could be added in future)
const (
	// EventTypeNormal Information only and will not cause any problems
	EventTypeNormal string = "Normal"
	// EventTypeWarning These events are to warn that something might go wrong
	EventTypeWarning string = "Warning"
)

// Event is a report of an event somewhere in the cluster.  Events
// have a limited retention time and triggers and messages may evolve
// with time.  Event consumers should not rely on the timing of an event
// with a given Reason reflecting a consistent underlying trigger, or the
// continued existence of events with that Reason.  Events should be
// treated as informative, best-effort, supplemental data.
type Event struct {
	TypeMeta `json:",inline"`
	// Standard object's metadata.
	ObjectMeta `json:",inline"`

	// The object that this event is about.
	InvolvedObject ObjectReference `json:"involvedObject"`

	// This should be a short, machine understandable string that gives the reason
	// for the transition into the object's current status.
	Reason string `json:"reason,omitempty"`

	// A human-readable description of the status of this operation.
	Message string `json:"message,omitempty"`

	// The component reporting this event. Should be a short machine understandable string.
	Source EventSource `json:"source,omitempty"`

	// The time at which the event was first recorded. (Time of server receipt is in TypeMeta.)
	FirstTimestamp time.Time `json:"firstTimestamp,omitempty"`

	// The time at which the most recent occurrence of this event was recorded.
	LastTimestamp *time.Time `json:"lastTimestamp,omitempty"`

	// The number of times this event has occurred.
	Count int32 `json:"count,omitempty"`

	// Type of this event (Normal, Warning), new types could be added in the future
	// +optional
	Type string `json:"type,omitempty"`

	// Time when this Event was first observed.
	EventTime *time.Time `json:"eventTime,omitempty"`

	// Data about the Event series this event represents or nil if it's a singleton Event.
	Series *EventSeries `json:"series,omitempty"`

	// What action was taken/failed regarding the Regarding object.
	Action string `json:"action,omitempty"`

	// Optional secondary object for more complex actions.
	Related *ObjectReference `json:"related,omitempty"`

	// Name of the controller that emitted this Event.
	ReportingController string `json:"reportingComponent"`

	// ID of the controller instance.
	ReportingInstance string `json:"reportingInstance"`

	// note is a human-readable description of the status of this operation.
	// Maximal length of the note is 1kB, but libraries should be prepared to
	// handle values up to 64kB.
	Note string `json:"note,omitempty"`

	// regarding contains the object this Event is about. In most cases it's an Object reporting controller
	// implements, e.g. ReplicaSetController implements ReplicaSets and this event is emitted because
	// it acts on some changes in a ReplicaSet object.
	Regarding ObjectReference `json:"regarding,omitempty"`
}

// EventSource contains information for an event.
type EventSource struct {
	// Component from which the event is generated.
	Component string `json:"component,omitempty"`
	// Node name on which the event is generated.
	Host string `json:"host,omitempty"`
}

type ObjectReference struct {
	// Kind of the referent.
	Kind string `json:"kind,omitempty"`
	// Name of the referent.
	Name string `json:"name,omitempty"`
	// UID of the referent.
	UID string `json:"uid,omitempty"`
	// API version of the referent.
	APIVersion string `json:"apiVersion,omitempty"`
	// Specific resourceVersion to which this reference is made, if any.
	ResourceVersion string `json:"resourceVersion,omitempty"`

	// If referring to a piece of an object instead of an entire object, this string
	// should contain a valid JSON/Go field access statement, such as desiredState.manifest.containers[2].
	// For example, if the object reference is to a container within a stage, this would take on a value like:
	// "spec.containers{name}" (where "name" refers to the name of the container that triggered
	// the event) or if no container name is specified "spec.containers[2]" (container with
	// index 2 in this stage). This syntax is chosen only to have some well-defined way of
	// referencing a part of an object.
	FieldPath string `json:"fieldPath,omitempty"`
}

func (m *ObjectReference) GetObjectKind() schema.ObjectKind {
	t := &TypeMeta{
		Kind:       m.Kind,
		APIVersion: m.APIVersion,
	}
	return t
}

func (m *ObjectReference) DeepCopyObject() runtime.Object {
	return m
}

// EventSeries contain information on series of events, i.e. thing that was/is happening
// continuously for some time.
type EventSeries struct {
	// Number of occurrences in this series up to the last heartbeat time
	Count int32 `json:"count,omitempty"`
	// Time of the last occurrence observed
	LastObservedTime *time.Time `json:"lastObservedTime,omitempty"`
}

func (m *Event) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Event) DeepCopyObject() runtime.Object {
	return m
}

type EventList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:",inline"`

	Items []Event `json:"items"`
}

func (m *EventList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *EventList) DeepCopyObject() runtime.Object {
	return m
}

type WatchEvent struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:",inline"`

	Type   string       `json:"type"`
	Object RawExtension `json:"object"`
}

func (m *WatchEvent) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *WatchEvent) DeepCopyObject() runtime.Object {
	a, _ := deepcopy.Anything(m)
	return a.(*WatchEvent)
}
