package dag

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
	// UpdateStatus update job state
	UpdateStatus(ctx context.Context, job *meta.Job) (*meta.Job, error)
	// GetDag get job's dag
	GetDag(ctx context.Context, workflowUID string) (*meta.Dag, error)
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

func (r *realJobControl) UpdateStatus(ctx context.Context, job *meta.Job) (*meta.Job, error) {
	return r.Client.CoreV1().Job().UpdateStatus(ctx, job, meta.UpdateOptions{})
}

func (r *realJobControl) GetDag(ctx context.Context, workflowUID string) (*meta.Dag, error) {
	return r.Client.CoreV1().Workflow().GetDag(ctx, workflowUID, meta.GetOptions{})
}

// stageControlInterface is an interface that knows how to update CronJob status
// created as an interface to allow testing.
type stageControlInterface interface {
	// CreateStage creates new Stage according to the spec.
	CreateStage(stage *meta.Stage) (*meta.Stage, error)
}

type realStageControl struct {
	Client client.Interface
}

func (r *realStageControl) CreateStage(stage *meta.Stage) (*meta.Stage, error) {
	return r.Client.CoreV1().Stage().Create(context.Background(), stage, meta.CreateOptions{})
}

type codeControlInterface interface {
	// GetCode retrieves a code.
	GetCode(ctx context.Context, name string) (*meta.Code, error)
	// GetCodes retrieves Code list.
	GetCodes(ctx context.Context) (*meta.CodeList, error)
}

type realCodeControl struct {
	Client client.Interface
}

var _ codeControlInterface = &realCodeControl{}

func (r *realCodeControl) GetCode(ctx context.Context, name string) (*meta.Code, error) {
	return r.Client.CoreV1().Code().Get(ctx, name, meta.GetOptions{})
}

func (r *realCodeControl) GetCodes(ctx context.Context) (*meta.CodeList, error) {
	return r.Client.CoreV1().Code().List(ctx, meta.ListOptions{})
}

type variableControlInterface interface {
	// GetVariable retrieves a Variable.
	GetVariable(ctx context.Context, name string) (*meta.Variable, error)
	// GetVariables retrieves Variable list.
	GetVariables(ctx context.Context) (*meta.VariableList, error)
}

type realVariableControl struct {
	Client client.Interface
}

var _ variableControlInterface = &realVariableControl{}

func (r *realVariableControl) GetVariable(ctx context.Context, name string) (*meta.Variable, error) {
	return r.Client.CoreV1().Variable().Get(ctx, name, meta.GetOptions{})
}

func (r *realVariableControl) GetVariables(ctx context.Context) (*meta.VariableList, error) {
	return r.Client.CoreV1().Variable().List(ctx, meta.ListOptions{})
}

type connectionControlInterface interface {
	// GetConnection retrieves a Connection.
	GetConnection(ctx context.Context, name string) (*meta.Connection, error)
	// GetConnections retrieves Connection list.
	GetConnections(ctx context.Context) (*meta.ConnectionList, error)
}

type realConnectionControl struct {
	Client client.Interface
}

var _ connectionControlInterface = &realConnectionControl{}

func (r *realConnectionControl) GetConnection(ctx context.Context, name string) (*meta.Connection, error) {
	return r.Client.CoreV1().Connection().Get(ctx, name, meta.GetOptions{})
}

func (r *realConnectionControl) GetConnections(ctx context.Context) (*meta.ConnectionList, error) {
	return r.Client.CoreV1().Connection().List(ctx, meta.ListOptions{})
}
