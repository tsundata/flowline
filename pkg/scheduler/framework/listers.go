package framework

// WorkerInfoLister interface represents anything that can list/get WorkerInfo objects from worker name.
type WorkerInfoLister interface {
	// List returns the list of WorkerInfos.
	List() ([]*WorkerInfo, error)
	// Get returns the WorkerInfo of the given worker name.
	Get(workerUID string) (*WorkerInfo, error)
}

// SharedLister groups scheduler-specific listers.
type SharedLister interface {
	WorkerInfos() WorkerInfoLister
}
