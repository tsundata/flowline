package workerruntime

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/scheduler/framework/plugins/names"
)

type WorkerRuntime struct{}

var _ framework.FilterPlugin = &WorkerRuntime{}
var _ framework.EnqueueExtensions = &WorkerRuntime{}

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = names.WorkerRuntime

	// ErrReason returned when node name doesn't match.
	ErrReason = "worker(s) didn't match the requested worker runtime"
)

func (pl *WorkerRuntime) EventsToRegister() []framework.ClusterEvent {
	return []framework.ClusterEvent{
		{Resource: framework.Worker, ActionType: framework.Add | framework.Update},
	}
}

func (pl *WorkerRuntime) Name() string {
	return Name
}

func (pl *WorkerRuntime) Filter(_ context.Context, _ *framework.CycleState, stage *meta.Stage, workerInfo *framework.WorkerInfo) *framework.Status {
	if workerInfo.Worker() == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}
	if !Fits(stage, workerInfo) {
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, ErrReason)
	}
	return nil
}

// Fits actually checks if the pod fits the node.
func Fits(stage *meta.Stage, workerInfo *framework.WorkerInfo) bool {
	if len(stage.Runtime) == 0 {
		return false
	}
	for _, rt := range workerInfo.Worker().Runtimes {
		if stage.Runtime == rt {
			return true
		}
	}
	return false
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, _ framework.Handle) (framework.Plugin, error) {
	return &WorkerRuntime{}, nil
}
