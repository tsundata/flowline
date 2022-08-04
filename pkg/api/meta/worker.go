package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type WorkerState string

const (
	WorkerReady   WorkerState = "ready"
	WorkerNoReady WorkerState = "no_ready"
	WorkerUnknown WorkerState = "unknown"
)

type Worker struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:",inline"`

	State    WorkerState `json:"state,omitempty"`
	Hostname string      `json:"hostname,omitempty"`
	Runtimes []string    `json:"runtimes,omitempty"`
}

func (m *Worker) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Worker) DeepCopyObject() runtime.Object {
	return m
}

type WorkerList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:",inline"`

	Items []Worker `json:"items"`
}

func (m *WorkerList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *WorkerList) DeepCopyObject() runtime.Object {
	return m
}
