package framework

import (
	"github.com/tsundata/flowline/pkg/api/meta"
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
