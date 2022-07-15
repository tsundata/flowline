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

	State    WorkerState
	Host     string   `json:"host,omitempty"`
	Runtimes []string `json:"runtimes,omitempty"`
}

func (m *Worker) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (m *Worker) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}

type WorkerList struct {
	TypeMeta
	ListMeta
	Items []Worker
}

func (m *WorkerList) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (m *WorkerList) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}
