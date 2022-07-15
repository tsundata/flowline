package framework

import (
	"errors"
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/util/flog"
	"sync/atomic"
	"time"
)

type StageInfo struct {
	Stage      *meta.Stage
	ParseError error
}

// QueuedStageInfo is a Pod wrapper with additional information related to
// the pod's status in the scheduling queue, such as the timestamp when
// it's added to the queue.
type QueuedStageInfo struct {
	*StageInfo
	// The time pod added to the scheduling queue.
	Timestamp time.Time
	// Number of schedule attempts before successfully scheduled.
	// It's used to record the # attempts metric.
	Attempts int
	// The time when the pod is added to the queue for the first time. The pod may be added
	// back to the queue multiple times before it's successfully scheduled.
	// It shouldn't be updated once initialized. It's used to record the e2e scheduling
	// latency for a pod.
	InitialAttemptTimestamp time.Time
	// If a Pod failed in a scheduling cycle, record the plugin names it failed by.
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
	// UpdateNodeXYZ is only applicable for Node events.
	UpdateNodeAllocatable // 100
	UpdateNodeLabel       // 1000
	UpdateNodeTaint       // 10000
	UpdateNodeCondition   // 100000

	All ActionType = 1<<iota - 1 // 111111

	// Use the general Update type if you don't either know or care the specific sub-Update type to use.
	Update = UpdateNodeAllocatable | UpdateNodeLabel | UpdateNodeTaint | UpdateNodeCondition
)

// GVK is short for group/version/kind, which can uniquely represent a particular API resource.
type GVK string

const (
	Stage    GVK = "Stage"
	Worker   GVK = "Worker"
	WildCard GVK = "*"
)

// ClusterEvent abstracts how a system resource's state gets changed.
// Resource represents the standard API resources such as Pod, Node, etc.
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

// NodeInfo is node level aggregated information.
type WorkerInfo struct {
	// Overall node information.
	worker *meta.Worker

	// Pods running on the node.
	Stages []*StageInfo

	// The subset of pods with affinity.
	StagesWithAffinity []*StageInfo

	// The subset of pods with required anti-affinity.
	StagesWithRequiredAntiAffinity []*StageInfo

	// Ports allocated on the node.
	UsedPorts interface{}

	// Total requested resources of all pods on this node. This includes assumed
	// pods, which scheduler has sent for binding, but may not be scheduled yet.
	Requested *Resource
	// Total requested resources of all pods on this node with a minimum value
	// applied to each container's CPU and memory requests. This does not reflect
	// the actual resource requests for this node, but is used to avoid scheduling
	// many zero-request pods onto one node.
	NonZeroRequested *Resource
	// We store allocatedResources (which is Node.Status.Allocatable.*) explicitly
	// as int64, to avoid conversions and accessing map.
	Allocatable *Resource

	// Whenever NodeInfo changes, generation is bumped.
	// This is used to avoid cloning it if the object didn't change.
	Generation int64
}

// Node returns overall information about this node.
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
	k, err := GetPodKey(stage)
	if err != nil {
		return err
	}

	for i := range n.Stages {
		k2, err := GetPodKey(n.Stages[i].Stage)
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
	return fmt.Errorf("no corresponding pod %s in pods of node %s", stage.UID, n.worker.UID)
}

func calculateResource(stage *meta.Stage) (res Resource, non0CPU int64, non0Mem int64) {
	// todo
	return
}

func NewStageInfo(stage *meta.Stage) *StageInfo {
	pInfo := &StageInfo{}
	pInfo.Update(stage)
	return pInfo
}

func (pi *StageInfo) Update(stage *meta.Stage) {
	if stage != nil && pi.Stage != nil && pi.Stage.UID == stage.UID {
		// PodInfo includes immutable information, and so it is safe to update the pod in place if it is
		// the exact same pod
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
	// We store allowedPodNumber (which is Node.Status.Allocatable.Pods().Value())
	// explicitly as int, to avoid conversions and improve performance.
	AllowedPodNumber int
}

// Diagnosis records the details to diagnose a scheduling failure.
type Diagnosis struct {
	WorkerToStatusMap    WorkerToStatusMap
	UnschedulablePlugins map[string]struct{}
	// PostFilterMsg records the messages returned from PostFilterPlugins.
	PostFilterMsg string
}

func GetPodKey(stage *meta.Stage) (string, error) {
	uid := stage.UID
	if len(uid) == 0 {
		return "", errors.New("cannot get cache key for pod with empty UID")
	}
	return uid, nil
}
