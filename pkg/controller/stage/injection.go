package stage

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/meta"
)

// stageControlInterface is an interface that knows how to update CronJob status
// created as an interface to allow testing.
type stageControlInterface interface {
	// GetStage retrieves a stage.
	GetStage(name string) (*meta.Stage, error)
	// GetStages retrieves list stage.
	GetStages(ctx context.Context) (*meta.StageList, error)
	// UpdateListStatus update job state
	UpdateListStatus(ctx context.Context, list *meta.StageList) (*meta.Status, error)
}

type realStageControl struct {
	Client client.Interface
}

func (r *realStageControl) GetStage(name string) (*meta.Stage, error) {
	return r.Client.CoreV1().Stage().Get(context.Background(), name, meta.GetOptions{})
}

func (r *realStageControl) GetStages(ctx context.Context) (*meta.StageList, error) {
	return r.Client.CoreV1().Stage().List(ctx, meta.ListOptions{})
}

func (r *realStageControl) UpdateListStatus(ctx context.Context, list *meta.StageList) (*meta.Status, error) {
	return r.Client.CoreV1().Stage().UpdateListStatus(ctx, list, meta.UpdateOptions{})
}
