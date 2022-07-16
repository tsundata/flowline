package framework

// NodeInfoLister interface represents anything that can list/get NodeInfo objects from node name.
type WorkerInfoLister interface {
	// List returns the list of NodeInfos.
	List() ([]*WorkerInfo, error)
	// HavePodsWithAffinityList returns the list of NodeInfos of nodes with pods with affinity terms.
	HavePodsWithAffinityList() ([]*WorkerInfo, error)
	// HavePodsWithRequiredAntiAffinityList returns the list of NodeInfos of nodes with pods with required anti-affinity terms.
	HavePodsWithRequiredAntiAffinityList() ([]*WorkerInfo, error)
	// Get returns the NodeInfo of the given node name.
	Get(nodeName string) (*WorkerInfo, error)
}

// SharedLister groups scheduler-specific listers.
type SharedLister interface {
	NodeInfos() WorkerInfoLister
}
