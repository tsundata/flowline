package crontrigger

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/meta"
)

// jobControlInterface is an interface that knows how to add or delete jobs
// created as an interface to allow testing.
type jobControlInterface interface {
	// GetJob retrieves a Job.
	GetJob(name string) (*meta.Job, error)
	// CreateJob creates new Jobs according to the spec.
	CreateJob(job *meta.Job) (*meta.Job, error)
	// UpdateJob updates a Job.
	UpdateJob(job *meta.Job) (*meta.Job, error)
	// PatchJob patches a Job.
	PatchJob(name string, pt string, data []byte, subresources ...string) (*meta.Job, error)
	// DeleteJob deletes the Job identified by name.
	DeleteJob(name string) error
}

type realJobControl struct {
	Client client.Interface
}

var _ jobControlInterface = &realJobControl{}

func (r *realJobControl) GetJob(name string) (*meta.Job, error) {
	return r.Client.CoreV1().Job().Get(context.Background(), name, meta.GetOptions{})
}

func (r *realJobControl) CreateJob(job *meta.Job) (*meta.Job, error) {
	return r.Client.CoreV1().Job().Create(context.Background(), job, meta.CreateOptions{})
}

func (r *realJobControl) UpdateJob(job *meta.Job) (*meta.Job, error) {
	return r.Client.CoreV1().Job().Update(context.Background(), job, meta.UpdateOptions{})
}

func (r *realJobControl) PatchJob(name string, pt string, data []byte, subresources ...string) (*meta.Job, error) {
	return r.Client.CoreV1().Job().Patch(context.Background(), name, pt, data, meta.PatchOptions{}, subresources...)
}

func (r *realJobControl) DeleteJob(name string) error {
	return r.Client.CoreV1().Job().Delete(context.Background(), name, meta.DeleteOptions{})
}

// cjControlInterface is an interface that knows how to update CronJob status
// created as an interface to allow testing.
type cjControlInterface interface {
	UpdateStatus(ctx context.Context, cj *meta.Workflow) (*meta.Workflow, error)
	// GetWorkflow retrieves a CronJob.
	GetWorkflow(ctx context.Context, name string) (*meta.Workflow, error)
}

type realCJControl struct {
	Client client.Interface
}

func (r *realCJControl) UpdateStatus(ctx context.Context, cj *meta.Workflow) (*meta.Workflow, error) {
	return r.Client.CoreV1().Workflow().UpdateStatus(ctx, cj, meta.UpdateOptions{})
}

func (r *realCJControl) GetWorkflow(ctx context.Context, name string) (*meta.Workflow, error) {
	return r.Client.CoreV1().Workflow().Get(ctx, name, meta.GetOptions{})
}
