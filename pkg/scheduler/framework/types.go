package framework

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/util/flog"
	"golang.org/x/xerrors"
	"sync/atomic"
	"time"
)

type StageInfo struct {
	Stage      *meta.Stage
	ParseError error
}

// QueuedStageInfo is a Stage wrapper with additional information related to
// the stage's status in the scheduling queue, such as the timestamp when
// it's added to the queue.
type QueuedStageInfo struct {
	*StageInfo
	// The time stage added to the scheduling queue.
	Timestamp time.Time
	// Number of schedule attempts before successfully scheduled.
	// It's used to record the # attempts metric.
	Attempts int
	// The time when the stage is added to the queue for the first time. The stage may be added
	// back to the queue multiple times before it's successfully scheduled.
	// It shouldn't be updated once initialized. It's used to record the e2e scheduling
	// latency for a stage.
	InitialAttemptTimestamp time.Time
	// If a Stage failed in a scheduling cycle, record the plugin names it failed by.
	UnschedulablePlugins map[string]struct{}
}

var generation int64

// ActionType is an integer to represent one type of resource change.
// Different ActionTypes can be bit-wised to compose new semantics.
type ActionType int64

// Constants for ActionTypes.
const (
	Add    ActionType = 1 << iota // 1
	Delete                        // 10
	// UpdateWorkerXYZ is only applicable for Worker events.
	UpdateWorkerAllocatable // 100
	UpdateWorkerLabel       // 1000
	UpdateWorkerTaint       // 10000
	UpdateWorkerCondition   // 100000

	All ActionType = 1<<iota - 1 // 111111

	// Use the general Update type if you don't either know or care the specific sub-Update type to use.
	Update = UpdateWorkerAllocatable | UpdateWorkerLabel | UpdateWorkerTaint | UpdateWorkerCondition
)

// GVK is short for group/version/kind, which can uniquely represent a particular API resource.
type GVK string

const (
	Stage    GVK = "Stage"
	Worker   GVK = "Worker"
	WildCard GVK = "*"
)

// ClusterEvent abstracts how a system resource's state gets changed.
// Resource represents the standard API resources such as Stage, Worker, etc.
// ActionType denotes the specific change such as Add, Update or Delete.
type ClusterEvent struct {
	Resource   GVK
	ActionType ActionType
	Label      string
}

// IsWildCard returns true if ClusterEvent follows WildCard semantics
func (ce ClusterEvent) IsWildCard() bool {
	return ce.Resource == WildCard && ce.ActionType == All
}

// WorkerInfo is worker level aggregated information.
type WorkerInfo struct {
	// Overall worker information.
	worker *meta.Worker

	// Stages running on the worker.
	Stages []*StageInfo

	// The subset of stages with affinity.
	StagesWithAffinity []*StageInfo

	// The subset of stages with required anti-affinity.
	StagesWithRequiredAntiAffinity []*StageInfo

	// Ports allocated on the worker.
	UsedPorts interface{}

	// Total requested resources of all stages on this worker. This includes assumed
	// stages, which scheduler has sent for binding, but may not be scheduled yet.
	Requested *Resource
	// Total requested resources of all stages on this worker with a minimum value
	// applied to each container's CPU and memory requests. This does not reflect
	// the actual resource requests for this worker, but is used to avoid scheduling
	// many zero-request stages onto one worker.
	NonZeroRequested *Resource
	// We store allocatedResources (which is Worker.Status.Allocatable.*) explicitly
	// as int64, to avoid conversions and accessing map.
	Allocatable *Resource

	// Whenever WorkerInfo changes, generation is bumped.
	// This is used to avoid cloning it if the object didn't change.
	Generation int64
}

// Worker returns overall information about this worker.
func (n *WorkerInfo) Worker() *meta.Worker {
	if n == nil {
		return nil
	}
	return n.worker
}

func (n *WorkerInfo) RemoveWorker() {
	n.worker = nil
	n.Generation = nextGeneration()
}

func (n *WorkerInfo) AddStage(stage *meta.Stage) {
	n.AddStageInfo(NewStageInfo(stage))
}

func (n *WorkerInfo) AddStageInfo(stageInfo *StageInfo) {
	res, non0CPU, non0Mem := calculateResource(stageInfo.Stage)
	n.Requested.MilliCPU += res.MilliCPU
	n.Requested.Memory += res.Memory
	n.Requested.EphemeralStorage += res.EphemeralStorage
	n.NonZeroRequested.MilliCPU += non0CPU
	n.NonZeroRequested.Memory += non0Mem
	n.Stages = append(n.Stages, stageInfo)

	n.Generation = nextGeneration()
}

func NewWorkerInfo(stages ...*meta.Stage) *WorkerInfo {
	ni := &WorkerInfo{
		Requested:        &Resource{},
		NonZeroRequested: &Resource{},
		Allocatable:      &Resource{},
		Generation:       nextGeneration(),
		UsedPorts:        nil,
	}
	for _, stage := range stages {
		ni.AddStage(stage)
	}
	return ni
}

func (n *WorkerInfo) SetWorker(worker *meta.Worker) {
	n.worker = worker
	// n.Allocatable = NewResource(worker.Status.Allocatable)
	n.Generation = nextGeneration()
}

func (n *WorkerInfo) Clone() *WorkerInfo {
	clone := &WorkerInfo{
		worker: n.worker,
		//Requested:        n.Requested.Clone(),
		//NonZeroRequested: n.NonZeroRequested.Clone(),
		//Allocatable:      n.Allocatable.Clone(),
		UsedPorts:  nil,
		Generation: n.Generation,
	}
	if len(n.Stages) > 0 {
		clone.Stages = append([]*StageInfo(nil), n.Stages...)
	}
	return clone
}

func (n *WorkerInfo) RemoveStage(stage *meta.Stage) error {
	k, err := GetStageKey(stage)
	if err != nil {
		return err
	}

	for i := range n.Stages {
		k2, err := GetStageKey(n.Stages[i].Stage)
		if err != nil {
			flog.Error(err)
			continue
		}
		if k == k2 {
			// delete the element
			n.Stages[i] = n.Stages[len(n.Stages)-1]
			n.Stages = n.Stages[:len(n.Stages)-1]
			// reduce the resource data
			res, non0CPU, non0Mem := calculateResource(stage)

			n.Requested.MilliCPU -= res.MilliCPU
			n.Requested.Memory -= res.Memory
			n.Requested.EphemeralStorage -= res.EphemeralStorage
			n.NonZeroRequested.MilliCPU -= non0CPU
			n.NonZeroRequested.Memory -= non0Mem

			n.Generation = nextGeneration()
			return nil
		}
	}
	return xerrors.Errorf("no corresponding stage %s in stages of worker %s", stage.UID, n.worker.UID)
}

func calculateResource(stage *meta.Stage) (res Resource, non0CPU int64, non0Mem int64) {
	return
}

func NewStageInfo(stage *meta.Stage) *StageInfo {
	pInfo := &StageInfo{}
	pInfo.Update(stage)
	return pInfo
}

func (pi *StageInfo) Update(stage *meta.Stage) {
	if stage != nil && pi.Stage != nil && pi.Stage.UID == stage.UID {
		// StageInfo includes immutable information, and so it is safe to update the stage in place if it is
		// the exact same stage
		pi.Stage = stage
		return
	}

	pi.Stage = stage
}

func nextGeneration() int64 {
	return atomic.AddInt64(&generation, 1)
}

// Resource is a collection of compute resource.
type Resource struct {
	MilliCPU         int64
	Memory           int64
	EphemeralStorage int64
	// We store allowedStageNumber (which is Worker.Status.Allocatable.Stages().Value())
	// explicitly as int, to avoid conversions and improve performance.
	AllowedStageNumber int
}

// Diagnosis records the details to diagnose a scheduling failure.
type Diagnosis struct {
	WorkerToStatusMap    WorkerToStatusMap
	UnschedulablePlugins map[string]struct{}
	// PostFilterMsg records the messages returned from PostFilterPlugins.
	PostFilterMsg string
}

func GetStageKey(stage *meta.Stage) (string, error) {
	uid := stage.UID
	if len(uid) == 0 {
		return "", xerrors.New("cannot get cache key for stage with empty UID")
	}
	return uid, nil
}
