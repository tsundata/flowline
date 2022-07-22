package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"time"
)

type TriggerType string

const (
	TriggerManual  TriggerType = "manual"
	TriggerCron    TriggerType = "cron"
	TriggerWebhook TriggerType = "webhook"
	TriggerWatch   TriggerType = "watch"
)

type Workflow struct {
	TypeMeta
	ObjectMeta

	Active                  bool        `json:"active,omitempty"`
	Trigger                 TriggerType `json:"trigger,omitempty"`
	TriggerParam            string      `json:"trigger_param,omitempty"`
	LastTriggerTimestamp    *time.Time  `json:"last_trigger_timestamp,omitempty"`
	CurrentTriggerTimestamp *time.Time  `json:"current_trigger_timestamp,omitempty"`
}

func (m *Workflow) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Workflow) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}

type WorkflowList struct {
	TypeMeta
	ListMeta
	Items []Workflow
}

func (m *WorkflowList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *WorkflowList) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}
