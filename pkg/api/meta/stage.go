package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type StageState string

const (
	StageCreate  StageState = "create"
	StageReady   StageState = "ready"
	StageSuccess StageState = "success"
	StageFailed  StageState = "failed"
)

type Stage struct {
	TypeMeta
	ObjectMeta

	SchedulerName string `json:"schedulerName,omitempty"`
	Priority      int    `json:"priority,omitempty"`
	WorkerUID     string `json:"workerUID,omitempty"`
	WorkerHost    string `json:"workerHost,omitempty"`

	WorkflowUID string `json:"workflowUID,omitempty"`
	JobUID      string `json:"jobUID,omitempty"`
	DagUID      string `json:"dagUID,omitempty"`

	State StageState `json:"state,omitempty"`

	Runtime     string       `json:"runtime,omitempty"`
	Code        string       `json:"code,omitempty"`
	Connections []Connection `json:"connections,omitempty"`
	Variables   []Variable   `json:"variables,omitempty"`

	DependNodeId []string `json:"dependNodeId,omitempty"`

	Input  interface{} `json:"input,omitempty"`
	Output interface{} `json:"output,omitempty"`
}

func (m *Stage) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Stage) DeepCopyObject() runtime.Object {
	return m
}

type StageList struct {
	TypeMeta
	ListMeta
	Items []Stage
}

func (m *StageList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *StageList) DeepCopyObject() runtime.Object {
	return m
}

type Binding struct {
	TypeMeta
	ObjectMeta

	Target Worker `json:"target,omitempty"`
}

func (m *Binding) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Binding) DeepCopyObject() runtime.Object {
	return m
}
