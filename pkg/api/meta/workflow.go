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
	TypeMeta   `json:",inline"`
	ObjectMeta `json:",inline"`

	Describe                string      `json:"describe,omitempty"`
	Active                  bool        `json:"active,omitempty"`
	Trigger                 TriggerType `json:"trigger,omitempty"`
	TriggerParam            string      `json:"triggerParam,omitempty"`
	LastTriggerTimestamp    *time.Time  `json:"lastTriggerTimestamp,omitempty"`
	CurrentTriggerTimestamp *time.Time  `json:"currentTriggerTimestamp,omitempty"`
	LastSuccessfulTimestamp *time.Time  `json:"LastSuccessfulTimestamp,omitempty"`

	StartingDeadlineSeconds    *int64 `json:"-"`
	FailedJobsHistoryLimit     *int32 `json:"-"`
	SuccessfulJobsHistoryLimit *int32 `json:"-"`
}

func (m *Workflow) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Workflow) DeepCopyObject() runtime.Object {
	return m
}

type WorkflowList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:",inline"`

	Items []Workflow `json:"items"`
}

func (m *WorkflowList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *WorkflowList) DeepCopyObject() runtime.Object {
	return m
}
