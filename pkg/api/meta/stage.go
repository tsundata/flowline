package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type Stage struct {
	TypeMeta
	ObjectMeta

	SchedulerName string
	WorkerHost    string

	JobUID string `json:"jobUID,omitempty"`
	DagUID string `json:"dagUID,omitempty"`

	State string `json:"state,omitempty"` // todo

	Runtime string `json:"runtime,omitempty"`
	Code    string `json:"code,omitempty"`

	Input  interface{} `json:"input,omitempty"`
	Output interface{} `json:"output,omitempty"`

	Connection []Connection `json:"connection,omitempty"`
	Variable   []Variable   `json:"variable,omitempty"`
}

func (m *Stage) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (m *Stage) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}
