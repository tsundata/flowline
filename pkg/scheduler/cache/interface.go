package cache

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
)

// Cache collects pods' information and provides node-level aggregated information.
// It's intended for generic scheduler to do efficient lookup.
// Cache's operations are pod centric. It does incremental updates based on pod events.
// Stage events are sent via network. We don't have guaranteed delivery of all events:
// We use Reflector to list and watch from remote.
// Reflector might be slow and do a relist, which would lead to missing events.
//
// State Machine of a pod's events in scheduler's cache:
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
// Note that an assumed pod can expire, because if we haven't received Add event notifying us
// for a while, there might be some problems and we shouldn't keep the pod in cache anymore.
//
// Note that "Initial", "Expired", and "Deleted" pods do not actually exist in cache.
// Based on existing use cases, we are making the following assumptions:
// - No pod would be assumed twice
// - A pod could be added without going through scheduler. In this case, we will see Add but not Assume event.
// - If a pod wasn't added, it wouldn't be removed or updated.
// - Both "Expired" and "Deleted" are valid end states. In case of some problems, e.g. network issue,
//   a pod might have changed its state (e.g. added and deleted) without delivering notification to the cache.
type Cache interface {
	// WorkerCount returns the number of nodes in the cache.
	// DO NOT use outside of tests.
	WorkerCount() int

	// StageCount returns the number of pods in the cache (including those from deleted nodes).
	// DO NOT use outside of tests.
	StageCount() (int, error)

	// AssumeStage assumes a pod scheduled and aggregates the pod's information into its node.
	// The implementation also decides the policy to expire pod before being confirmed (receiving Add event).
	// After expiration, its information would be subtracted.
	AssumeStage(stage *meta.Stage) error

	// FinishBinding signals that cache for assumed pod can be expired
	FinishBinding(stage *meta.Stage) error

	// ForgetStage removes an assumed pod from cache.
	ForgetStage(stage *meta.Stage) error

	// AddStage either confirms a pod if it's assumed, or adds it back if it's expired.
	// If added back, the pod's information would be added again.
	AddStage(stage *meta.Stage) error

	// UpdateStage removes oldStage's information and adds newStage's information.
	UpdateStage(oldStage, newStage *meta.Stage) error

	// RemoveStage removes a pod. The pod's information would be subtracted from assigned node.
	RemoveStage(stage *meta.Stage) error

	// GetStage returns the pod from the cache with the same namespace and the
	// same name of the specified pod.
	GetStage(stage *meta.Stage) (*meta.Stage, error)

	// IsAssumedStage returns true if the pod is assumed and not expired.
	IsAssumedStage(stage *meta.Stage) (bool, error)

	// AddWorker adds overall information about node.
	// It returns a clone of added WorkerInfo object.
	AddWorker(worker *meta.Worker) *framework.WorkerInfo

	// UpdateWorker updates overall information about node.
	// It returns a clone of updated WorkerInfo object.
	UpdateWorker(oldWorker, newWorker *meta.Worker) *framework.WorkerInfo

	// RemoveWorker removes overall information about node.
	RemoveWorker(worker *meta.Worker) error

	// UpdateSnapshot updates the passed infoSnapshot to the current contents of Cache.
	// The node info contains aggregated information of pods scheduled (including assumed to be)
	// on this node.
	// The snapshot only includes Workers that are not deleted at the time this function is called.
	// nodeinfo.Worker() is guaranteed to be not nil for all the nodes in the snapshot.
	UpdateSnapshot(workerSnapshot *Snapshot) error

	// Dump produces a dump of the current cache.
	Dump() *Dump
}

// Dump is a dump of the cache state.
type Dump struct {
	AssumedStages map[string]struct{}
	Workers       map[string]*framework.WorkerInfo
}
