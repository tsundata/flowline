package defaultbinder

import (
	"context"
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/scheduler/framework/plugins/names"
	"github.com/tsundata/flowline/pkg/util/flog"
)

// Name of the plugin used in the plugin registry and configurations.
const Name = names.DefaultBinder

// DefaultBinder binds pods to nodes using a k8s client.
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

// Bind binds pods to nodes using the k8s client.
func (b DefaultBinder) Bind(ctx context.Context, state *framework.CycleState, p *meta.Stage, nodeName string) *framework.Status {
	flog.Infof("Attempting to bind pod to node, %v, %s", p, nodeName)
	binding := &meta.Binding{
		ObjectMeta: meta.ObjectMeta{Name: p.Name, UID: p.UID},
		Target:     &meta.Worker{ObjectMeta: meta.ObjectMeta{UID: nodeName}},
	}
	fmt.Println(binding)
	//err := b.handle.ClientSet().CoreV1().Pods(binding.Namespace).Bind(ctx, binding, metav1.CreateOptions{}) fixme
	//if err != nil {
	//		return framework.AsStatus(err)
	//	}
	return nil
}
