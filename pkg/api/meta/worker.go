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
	TypeMeta
	ObjectMeta

	State    WorkerState `json:"state,omitempty"`
	Host     string      `json:"host,omitempty"`
	Runtimes []string    `json:"runtimes,omitempty"`
}

func (m *Worker) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Worker) DeepCopyObject() runtime.Object {
	return m
}

type WorkerList struct {
	TypeMeta
	ListMeta
	Items []Worker
}

func (m *WorkerList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *WorkerList) DeepCopyObject() runtime.Object {
	return m
}
