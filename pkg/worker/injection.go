package worker

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/meta"
)

// stageControlInterface is an interface that knows how to update CronJob status
// created as an interface to allow testing.
type stageControlInterface interface {
	// UpdateStage updates a Stage.
	UpdateStage(ctx context.Context, stage *meta.Stage) (*meta.Stage, error)
}

type realStageControl struct {
	Client client.Interface
}

func (r *realStageControl) UpdateStage(ctx context.Context, stage *meta.Stage) (*meta.Stage, error) {
	return r.Client.CoreV1().Stage().Update(ctx, stage, meta.UpdateOptions{})
}

type workerControlInterface interface {
	Register(ctx context.Context, worker *meta.Worker) (*meta.Worker, error)
	Heartbeat(ctx context.Context, worker *meta.Worker) (*meta.Status, error)
}

type realWorkerControl struct {
	Client client.Interface
}

func (r *realWorkerControl) Register(ctx context.Context, worker *meta.Worker) (*meta.Worker, error) {
	return r.Client.CoreV1().Worker().Register(ctx, worker, meta.CreateOptions{})
}

func (r *realWorkerControl) Heartbeat(ctx context.Context, worker *meta.Worker) (*meta.Status, error) {
	return r.Client.CoreV1().Worker().Heartbeat(ctx, worker, meta.UpdateOptions{})
}
