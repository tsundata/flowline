package config

import "time"

// Extender holds the parameters used to communicate with the extender. If a verb is unspecified/empty,
// it is assumed that the extender chose not to provide that extension.
type Extender struct {
	// URLPrefix at which the extender is available
	URLPrefix string
	// Verb for the filter call, empty if not supported. This verb is appended to the URLPrefix when issuing the filter call to extender.
	FilterVerb string
	// Verb for the preempt call, empty if not supported. This verb is appended to the URLPrefix when issuing the preempt call to extender.
	PreemptVerb string
	// Verb for the prioritize call, empty if not supported. This verb is appended to the URLPrefix when issuing the prioritize call to extender.
	PrioritizeVerb string
	// The numeric multiplier for the worker scores that the prioritize call generates.
	// The weight should be a positive integer
	Weight int64
	// Verb for the bind call, empty if not supported. This verb is appended to the URLPrefix when issuing the bind call to extender.
	// If this method is implemented by the extender, it is the extender's responsibility to bind the stage to apiserver. Only one extender
	// can implement this function.
	BindVerb string
	// EnableHTTPS specifies whether https should be used to communicate with the extender
	EnableHTTPS bool

	// HTTPTimeout specifies the timeout duration for a call to the extender. Filter timeout fails the scheduling of the stage. Prioritize
	// timeout is ignored, k8s/other extenders priorities are used to select the worker.
	HTTPTimeout time.Duration
	// WorkerCacheCapable specifies that the extender is capable of caching worker information,
	// so the scheduler should only send minimal information about the eligible workers
	// assuming that the extender already cached full details of all workers in the cluster
	WorkerCacheCapable bool
	// ManagedResources is a list of extended resources that are managed by
	// this extender.
	// - A stage will be sent to the extender on the Filter, Prioritize and Bind
	//   (if the extender is the binder) phases iff the stage requests at least
	//   one of the extended resources in this list. If empty or unspecified,
	//   all stages will be sent to this extender.
	// - If IgnoredByScheduler is set to true for a resource, kube-scheduler
	//   will skip checking the resource in predicates.
	// +optional
	ManagedResources []string
	// Ignorable specifies if the extender is ignorable, i.e. scheduling should not
	// fail when the extender returns an error or is not reachable.
	Ignorable bool
}
