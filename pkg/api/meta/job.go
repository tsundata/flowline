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

	State             JobState   `json:"state,omitempty"`
	TriggerTimestamp  *time.Time `json:"triggerTimestamp,omitempty"`
	ScheduleTimestamp *time.Time `json:"scheduleTimestamp,omitempty"`
}

func (m *Job) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (m *Job) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}

type JobList struct {
	TypeMeta
	ListMeta
	Items []Job
}

func (m *JobList) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (m *JobList) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}
