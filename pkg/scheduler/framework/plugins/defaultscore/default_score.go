package defaultscore

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/scheduler/framework/plugins/names"
)

const Name = names.DefaultScore

type DefaultScore struct{}

var _ framework.ScorePlugin = &DefaultScore{}
var _ framework.ScoreExtensions = &DefaultScore{}

func (d *DefaultScore) Name() string {
	return Name
}

func (d *DefaultScore) Score(ctx context.Context, state *framework.CycleState, p *meta.Stage, workerName string) (int64, *framework.Status) {
	return 1, nil
}

func (d *DefaultScore) ScoreExtensions() framework.ScoreExtensions {
	return d
}

func (d *DefaultScore) NormalizeScore(ctx context.Context, state *framework.CycleState, p *meta.Stage, scores framework.WorkerScoreList) *framework.Status {
	return nil
}

func New(_ runtime.Object, _ framework.Handle) (framework.Plugin, error) {
	return &DefaultScore{}, nil
}
