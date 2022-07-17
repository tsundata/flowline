package framework

import (
	"github.com/tsundata/flowline/pkg/api/meta"
)

// HostPriority represents the priority of scheduling to a particular host, higher priority is better.
type HostPriority struct {
	// Name of the host
	Host string
	// Score associated with the host
	Score int64
}

// HostPriorityList declares a []HostPriority type.
type HostPriorityList []HostPriority

// Extender is an interface for external processes to influence scheduling
// decisions made by Kubernetes. This is typically needed for resources not directly
// managed by Kubernetes.
type Extender interface {
	// Name returns a unique name that identifies the extender.
	Name() string

	// Filter based on extender-implemented predicate functions. The filtered list is
	// expected to be a subset of the supplied list.
	// The failedNodes and failedAndUnresolvableNodes optionally contains the list
	// of failed nodes and failure reasons, except nodes in the latter are
	// unresolvable.
	Filter(stage *meta.Stage, workers []*meta.Worker) (filteredWorkers []*meta.Worker, failedNodesMap map[string]string, failedAndUnresolvable map[string]string, err error)

	// Prioritize based on extender-implemented priority functions. The returned scores & weight
	// are used to compute the weighted score for an extender. The weighted scores are added to
	// the scores computed by Kubernetes scheduler. The total scores are used to do the host selection.
	Prioritize(stage *meta.Stage, workers []*meta.Worker) (hostPriorities *HostPriorityList, weight int64, err error)

	// Bind delegates the action of binding a pod to a node to the extender.
	Bind(binding *meta.Binding) error

	// IsBinder returns whether this extender is configured for the Bind method.
	IsBinder() bool

	// IsInterested returns true if at least one extended resource requested by
	// this pod is managed by this extender.
	IsInterested(stage *meta.Stage) bool

	// ProcessPreemption returns nodes with their victim pods processed by extender based on
	// given:
	//   1. Pod to schedule
	//   2. Candidate nodes and victim pods (nodeNameToVictims) generated by previous scheduling process.
	// The possible changes made by extender may include:
	//   1. Subset of given candidate nodes after preemption phase of extender.
	//   2. A different set of victim pod for every given candidate node after preemption phase of extender.
	ProcessPreemption(
		stage *meta.Stage,
		nodeNameToVictims map[string]interface{},
		workerInfos interface{},
	) (map[string]interface{}, error)

	// SupportsPreemption returns if the scheduler extender support preemption or not.
	SupportsPreemption() bool

	// IsIgnorable returns true indicates scheduling should not fail when this extender
	// is unavailable. This gives scheduler ability to fail fast and tolerate non-critical extenders as well.
	IsIgnorable() bool
}
