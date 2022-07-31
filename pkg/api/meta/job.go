package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"time"
)

type JobState string

const (
	JobCreate          JobState = "create"
	JobFailed          JobState = "failed"
	JobQueued          JobState = "queued"
	JobRunning         JobState = "running"
	JobScheduled       JobState = "scheduled"
	JobSkipped         JobState = "skipped"
	JobSuccess         JobState = "success"
	JobUpForReschedule JobState = "up_for_reschedule"
	JobUpForRetry      JobState = "up_for_retry"
	JobUpstreamFailed  JobState = "upstream_failed"
)

type Job struct {
	TypeMeta
	ObjectMeta

	WorkflowUID         string     `json:"workflowUID"`
	State               JobState   `json:"state,omitempty"`
	TriggerTimestamp    *time.Time `json:"triggerTimestamp,omitempty"`
	ScheduleTimestamp   *time.Time `json:"scheduleTimestamp,omitempty"`
	CompletionTimestamp *time.Time `json:"completionTimestamp,omitempty"`

	StartTime *time.Time `json:"-"`
}

func (m *Job) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Job) DeepCopyObject() runtime.Object {
	return m
}

type JobList struct {
	TypeMeta
	ListMeta
	Items []Job
}

func (m *JobList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *JobList) DeepCopyObject() runtime.Object {
	return m
}
