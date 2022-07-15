package plugins

import (
	"github.com/tsundata/flowline/pkg/scheduler/framework/plugins/workerruntime"
	"github.com/tsundata/flowline/pkg/scheduler/framework/runtime"
)

func NewInTreeRegistry() runtime.Registry {
	return runtime.Registry{
		workerruntime.Name: workerruntime.New,
	}
}
