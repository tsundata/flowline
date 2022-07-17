package defaultpermit

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/scheduler/framework/plugins/names"
	"time"
)

const Name = names.DefaultPermit

type DefaultPermit struct{}

var _ framework.PermitPlugin = &DefaultPermit{}

func (d *DefaultPermit) Name() string {
	return Name
}

func (d *DefaultPermit) Permit(ctx context.Context, state *framework.CycleState, p *meta.Stage, workerUID string) (*framework.Status, time.Duration) {
	return nil, 0
}

func New(_ runtime.Object, _ framework.Handle) (framework.Plugin, error) {
	return &DefaultPermit{}, nil
}
