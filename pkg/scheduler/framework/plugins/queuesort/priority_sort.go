package queuesort

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/scheduler/framework/plugins/names"
)

const Name = names.PrioritySort

type PrioritySort struct{}

var _ framework.QueueSortPlugin = &PrioritySort{}

func (p *PrioritySort) Name() string {
	return Name
}

func (p *PrioritySort) Less(info1 *framework.QueuedStageInfo, info2 *framework.QueuedStageInfo) bool {
	p1 := info1.Stage.Priority
	p2 := info2.Stage.Priority
	return (p1 > p2) || (p1 == p2 && info1.Timestamp.Before(info2.Timestamp))
}

func New(_ runtime.Object, _ framework.Handle) (framework.Plugin, error) {
	return &PrioritySort{}, nil
}
