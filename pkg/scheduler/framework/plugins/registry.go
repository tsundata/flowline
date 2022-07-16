package plugins

import (
	"github.com/tsundata/flowline/pkg/scheduler/framework/plugins/defaultbinder"
	"github.com/tsundata/flowline/pkg/scheduler/framework/plugins/defaultpermit"
	"github.com/tsundata/flowline/pkg/scheduler/framework/plugins/defaultscore"
	"github.com/tsundata/flowline/pkg/scheduler/framework/plugins/queuesort"
	"github.com/tsundata/flowline/pkg/scheduler/framework/plugins/workerruntime"
	"github.com/tsundata/flowline/pkg/scheduler/framework/runtime"
)

func NewInTreeRegistry() runtime.Registry {
	return runtime.Registry{
		workerruntime.Name: workerruntime.New,
		queuesort.Name:     queuesort.New,
		defaultscore.Name:  defaultscore.New,
		defaultpermit.Name: defaultpermit.New,
		defaultbinder.Name: defaultbinder.New,
	}
}
