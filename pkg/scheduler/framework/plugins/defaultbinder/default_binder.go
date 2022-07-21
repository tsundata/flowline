package defaultbinder

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/scheduler/framework/plugins/names"
	"github.com/tsundata/flowline/pkg/util/flog"
)

// Name of the plugin used in the plugin registry and configurations.
const Name = names.DefaultBinder

// DefaultBinder binds stages to workers using a k8s client.
type DefaultBinder struct {
	handle framework.Handle
}

var _ framework.BindPlugin = &DefaultBinder{}

// New creates a DefaultBinder.
func New(_ runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	return &DefaultBinder{handle: handle}, nil
}

// Name returns the name of the plugin.
func (b DefaultBinder) Name() string {
	return Name
}

// Bind binds stages to workers using the k8s client.
func (b DefaultBinder) Bind(ctx context.Context, state *framework.CycleState, p *meta.Stage, workerUID string) *framework.Status {
	flog.Infof("Attempting to bind stage to worker, %s %s : %s", p.Name, p.UID, workerUID)
	binding := &meta.Binding{
		ObjectMeta: meta.ObjectMeta{Name: p.Name, UID: p.UID},
		Target:     &meta.Worker{ObjectMeta: meta.ObjectMeta{UID: workerUID}},
	}
	err := b.handle.ClientSet().CoreV1().Stage().Bind(ctx, binding, meta.CreateOptions{})
	if err != nil {
		return framework.AsStatus(err)
	}
	return nil
}
