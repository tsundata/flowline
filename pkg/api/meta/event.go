package meta

import (
	"github.com/barkimedes/go-deepcopy"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type Event struct {
	TypeMeta
	ObjectMeta
}

func (m *Event) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Event) DeepCopyObject() runtime.Object {
	return m
}

type EventList struct {
	TypeMeta
	ListMeta
	Items []Event
}

func (m *EventList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *EventList) DeepCopyObject() runtime.Object {
	return m
}

type WatchEvent struct {
	TypeMeta
	ObjectMeta

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
