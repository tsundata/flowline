package cache

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
)

// Cache collects stages' information and provides worker-level aggregated information.
// It's intended for generic scheduler to do efficient lookup.
// Cache's operations are stage centric. It does incremental updates based on stage events.
// Stage events are sent via network. We don't have guaranteed delivery of all events:
// We use Reflector to list and watch from remote.
// Reflector might be slow and do a relist, which would lead to missing events.
//
// State Machine of a stage's events in scheduler's cache:
//
//
//   +-------------------------------------------+  +----+
//   |                            Add            |  |    |
//   |                                           |  |    | Update
//   +      Assume                Add            v  v    |
//Initial +--------> Assumed +------------+---> Added <--+
//   ^                +   +               |       +
//   |                |   |               |       |
//   |                |   |           Add |       | Remove
//   |                |   |               |       |
//   |                |   |               +       |
//   +----------------+   +-----------> Expired   +----> Deleted
//         Forget             Expire
//
//
// Note that an assumed stage can expire, because if we haven't received Add event notifying us
// for a while, there might be some problems and we shouldn't keep the stage in cache anymore.
//
// Note that "Initial", "Expired", and "Deleted" stages do not actually exist in cache.
// Based on existing use cases, we are making the following assumptions:
// - No stage would be assumed twice
// - A stage could be added without going through scheduler. In this case, we will see Add but not Assume event.
// - If a stage wasn't added, it wouldn't be removed or updated.
// - Both "Expired" and "Deleted" are valid end states. In case of some problems, e.g. network issue,
//   a stage might have changed its state (e.g. added and deleted) without delivering notification to the cache.
type Cache interface {
	// WorkerCount returns the number of workers in the cache.
	// DO NOT use outside of tests.
	WorkerCount() int

	// StageCount returns the number of stages in the cache (including those from deleted workers).
	// DO NOT use outside of tests.
	StageCount() (int, error)

	// AssumeStage assumes a stage scheduled and aggregates the stage's information into its worker.
	// The implementation also decides the policy to expire stage before being confirmed (receiving Add event).
	// After expiration, its information would be subtracted.
	AssumeStage(stage *meta.Stage) error

	// FinishBinding signals that cache for assumed stage can be expired
	FinishBinding(stage *meta.Stage) error

	// ForgetStage removes an assumed stage from cache.
	ForgetStage(stage *meta.Stage) error

	// AddStage either confirms a stage if it's assumed, or adds it back if it's expired.
	// If added back, the stage's information would be added again.
	AddStage(stage *meta.Stage) error

	// UpdateStage removes oldStage's information and adds newStage's information.
	UpdateStage(oldStage, newStage *meta.Stage) error

	// RemoveStage removes a stage. The stage's information would be subtracted from assigned worker.
	RemoveStage(stage *meta.Stage) error

	// GetStage returns the stage from the cache with the same namespace and the
	// same name of the specified stage.
	GetStage(stage *meta.Stage) (*meta.Stage, error)

	// IsAssumedStage returns true if the stage is assumed and not expired.
	IsAssumedStage(stage *meta.Stage) (bool, error)

	// AddWorker adds overall information about worker.
	// It returns a clone of added WorkerInfo object.
	AddWorker(worker *meta.Worker) *framework.WorkerInfo

	// UpdateWorker updates overall information about worker.
	// It returns a clone of updated WorkerInfo object.
	UpdateWorker(oldWorker, newWorker *meta.Worker) *framework.WorkerInfo

	// RemoveWorker removes overall information about worker.
	RemoveWorker(worker *meta.Worker) error

	// UpdateSnapshot updates the passed infoSnapshot to the current contents of Cache.
	// The worker info contains aggregated information of stages scheduled (including assumed to be)
	// on this worker.
	// The snapshot only includes Workers that are not deleted at the time this function is called.
	// workerinfo.Worker() is guaranteed to be not nil for all the workers in the snapshot.
	UpdateSnapshot(workerSnapshot *Snapshot) error

	// Dump produces a dump of the current cache.
	Dump() *Dump
}

// Dump is a dump of the cache state.
type Dump struct {
	AssumedStages map[string]struct{}
	Workers       map[string]*framework.WorkerInfo
}
