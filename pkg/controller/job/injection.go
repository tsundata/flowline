package job

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/meta"
)

// stageControlInterface is an interface that knows how to update CronJob status
// created as an interface to allow testing.
type jobControlInterface interface {
	// GetJob retrieves a job.
	GetJob(name string) (*meta.Job, error)
	// UpdateStatus update job state
	UpdateStatus(ctx context.Context, job *meta.Job) (*meta.Job, error)
}

type realJobControl struct {
	Client client.Interface
}

func (r *realJobControl) GetJob(name string) (*meta.Job, error) {
	return r.Client.CoreV1().Job().Get(context.Background(), name, meta.GetOptions{})
}

func (r *realJobControl) UpdateStatus(ctx context.Context, job *meta.Job) (*meta.Job, error) {
	return r.Client.CoreV1().Job().UpdateStatus(ctx, job, meta.UpdateOptions{})
}

// stageControlInterface is an interface that knows how to update CronJob status
// created as an interface to allow testing.
type stageControlInterface interface {
	// GetStage retrieves a stage.
	GetStage(name string) (*meta.Stage, error)
	// GetStages retrieves list stage.
	GetStages(ctx context.Context) (*meta.StageList, error)
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
