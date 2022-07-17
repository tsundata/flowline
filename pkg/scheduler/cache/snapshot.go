package cache

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
)

// Snapshot is a snapshot of cache WorkerInfo and WorkerTree order. The scheduler takes a
// snapshot at the beginning of each scheduling cycle and uses it for its operations in that cycle.
type Snapshot struct {
	// workerInfoMap a map of worker name to a snapshot of its WorkerInfo.
	workerInfoMap map[string]*framework.WorkerInfo
	// workerInfoList is the list of workers as ordered in the cache's workerTree.
	workerInfoList []*framework.WorkerInfo
	// haveStagesWithAffinityWorkerInfoList is the list of workers with at least one stage declaring affinity terms.
	haveStagesWithAffinityWorkerInfoList []*framework.WorkerInfo
	// haveStagesWithRequiredAntiAffinityWorkerInfoList is the list of workers with at least one stage declaring
	// required anti-affinity terms.
	haveStagesWithRequiredAntiAffinityWorkerInfoList []*framework.WorkerInfo

	generation int64
}

var _ framework.SharedLister = &Snapshot{}

// NewEmptySnapshot initializes a Snapshot struct and returns it.
func NewEmptySnapshot() *Snapshot {
	return &Snapshot{
		workerInfoMap: make(map[string]*framework.WorkerInfo),
	}
}

// NewSnapshot initializes a Snapshot struct and returns it.
func NewSnapshot(stages []*meta.Stage, workers []*meta.Worker) *Snapshot {
	workerInfoMap := createWorkerInfoMap(stages, workers)
	workerInfoList := make([]*framework.WorkerInfo, 0, len(workerInfoMap))
	haveStagesWithAffinityWorkerInfoList := make([]*framework.WorkerInfo, 0, len(workerInfoMap))
	haveStagesWithRequiredAntiAffinityWorkerInfoList := make([]*framework.WorkerInfo, 0, len(workerInfoMap))
	for _, v := range workerInfoMap {
		workerInfoList = append(workerInfoList, v)
	}

	s := NewEmptySnapshot()
	s.workerInfoMap = workerInfoMap
	s.workerInfoList = workerInfoList
	s.haveStagesWithAffinityWorkerInfoList = haveStagesWithAffinityWorkerInfoList
	s.haveStagesWithRequiredAntiAffinityWorkerInfoList = haveStagesWithRequiredAntiAffinityWorkerInfoList

	return s
}

// createWorkerInfoMap obtains a list of stages and pivots that list into a map
// where the keys are worker names and the values are the aggregated information
// for that worker.
func createWorkerInfoMap(stages []*meta.Stage, workers []*meta.Worker) map[string]*framework.WorkerInfo {
	workerNameToInfo := make(map[string]*framework.WorkerInfo)
	for _, stage := range stages {
		workerName := stage.WorkerUID
		if _, ok := workerNameToInfo[workerName]; !ok {
			workerNameToInfo[workerName] = framework.NewWorkerInfo()
		}
		workerNameToInfo[workerName].AddStage(stage)
	}

	for _, worker := range workers {
		if _, ok := workerNameToInfo[worker.UID]; !ok {
			workerNameToInfo[worker.UID] = framework.NewWorkerInfo()
		}
		workerInfo := workerNameToInfo[worker.UID]
		workerInfo.SetWorker(worker)
	}
	return workerNameToInfo
}

// WorkerInfos returns a WorkerInfoLister.
func (s *Snapshot) WorkerInfos() framework.WorkerInfoLister {
	return s
}

// NumWorkers returns the number of workers in the snapshot.
func (s *Snapshot) NumWorkers() int {
	return len(s.workerInfoList)
}

// List returns the list of workers in the snapshot.
func (s *Snapshot) List() ([]*framework.WorkerInfo, error) {
	return s.workerInfoList, nil
}

// Get returns the WorkerInfo of the given worker name.
func (s *Snapshot) Get(workerName string) (*framework.WorkerInfo, error) {
	if v, ok := s.workerInfoMap[workerName]; ok && v.Worker() != nil {
		return v, nil
	}
	return nil, fmt.Errorf("workerinfo not found for worker name %q", workerName)
}
