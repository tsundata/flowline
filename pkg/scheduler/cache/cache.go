package cache

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"os"
	"sync"
	"time"
)

var (
	cleanAssumedPeriod = 1 * time.Second
)

func New(ttl time.Duration, stop <-chan struct{}) Cache {
	cache := newCache(ttl, cleanAssumedPeriod, stop)
	cache.run()
	return cache
}

func newCache(ttl, period time.Duration, stop <-chan struct{}) *cacheImpl {
	return &cacheImpl{
		ttl:    ttl,
		period: period,
		stop:   stop,

		workers:       make(map[string]*workerInfoListItem),
		workerTree:    newWorkerTree(nil),
		assumedStages: make(map[string]struct{}),
		stageStates:   make(map[string]*stageState),
	}
}

type cacheImpl struct {
	stop   <-chan struct{}
	ttl    time.Duration
	period time.Duration

	// This mutex guards all fields within this cache struct.
	mu sync.RWMutex
	// a set of assumed stage keys.
	// The key could further be used to get an entry in stageStates.
	assumedStages map[string]struct{}
	// a map from stage key to stageState.
	stageStates map[string]*stageState
	workers     map[string]*workerInfoListItem
	// headWorker points to the most recently updated WorkerInfo in "workers". It is the
	// head of the linked list.
	headWorker *workerInfoListItem
	workerTree *workerTree
}

func (c *cacheImpl) WorkerCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.workers)
}

func (c *cacheImpl) StageCount() (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for _, n := range c.workers {
		count += len(n.info.Stages)
	}
	return count, nil
}

func (c *cacheImpl) AssumeStage(stage *meta.Stage) error {
	key, err := framework.GetStageKey(stage)
	if err != nil {
		return err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if _, ok := c.stageStates[key]; ok {
		return fmt.Errorf("stage %v is in the cache, so can't be assumed", key)
	}

	return c.addStage(stage, true)
}

func (c *cacheImpl) FinishBinding(stage *meta.Stage) error {
	return c.finishBinding(stage, time.Now())
}

func (c *cacheImpl) finishBinding(stage *meta.Stage, now time.Time) error {
	key, err := framework.GetStageKey(stage)
	if err != nil {
		return err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	flog.Infof("Finished binding for stage, can be expired, %s %s, worker %s %s", stage.Name, stage.UID, stage.WorkerUID, stage.WorkerHost)
	currState, ok := c.stageStates[key]
	_, has := c.assumedStages[key]
	if ok && has {
		if c.ttl == time.Duration(0) {
			currState.deadline = nil
		} else {
			dl := now.Add(c.ttl)
			currState.deadline = &dl
		}
		currState.bindingFinished = true
	}
	return nil
}

func (c *cacheImpl) ForgetStage(stage *meta.Stage) error {
	key, err := framework.GetStageKey(stage)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	currState, ok := c.stageStates[key]
	if ok && currState.stage.WorkerUID != stage.WorkerUID {
		return fmt.Errorf("stage %v was assumed on %v but assigned to %v", key, stage.WorkerUID, currState.stage.WorkerUID)
	}

	// Only assumed stage can be forgotten.
	_, has := c.assumedStages[key]
	if ok && has {
		return c.removeStage(stage)
	}
	return fmt.Errorf("stage %v wasn't assumed so cannot be forgotten", key)
}

func (c *cacheImpl) AddStage(stage *meta.Stage) error {
	key, err := framework.GetStageKey(stage)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	currState, ok := c.stageStates[key]
	_, has := c.assumedStages[key]
	switch {
	case ok && has:
		if currState.stage.WorkerUID != stage.WorkerUID {
			// The stage was added to a different worker than it was assumed to.
			flog.Infof("Stage was added to a different worker than it was assumed, %v", stage)
			if err = c.updateStage(currState.stage, stage); err != nil {
				flog.Errorf("%s Error occurred while updating stage", err)
			}
		} else {
			delete(c.assumedStages, key)
			c.stageStates[key].deadline = nil
			c.stageStates[key].stage = stage
		}
	case !ok:
		// Stage was expired. We should add it back.
		if err = c.addStage(stage, false); err != nil {
			flog.Errorf("Error occurred while adding stage %s", err)
		}
	default:
		return fmt.Errorf("stage %v was already in added state", key)
	}
	return nil
}

func (c *cacheImpl) UpdateStage(oldStage, newStage *meta.Stage) error {
	key, err := framework.GetStageKey(oldStage)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	currState, ok := c.stageStates[key]
	// An assumed stage won't have Update/Remove event. It needs to have Add event
	// before Update event, in which case the state would change from Assumed to Added.
	_, has := c.assumedStages[key]
	if ok && !has {
		if currState.stage.WorkerUID != newStage.WorkerUID {
			flog.Errorf("Stage updated on a different worker than previously added to %+v", oldStage)
			flog.Errorf("scheduler cache is corrupted and can badly affect scheduling decisions")
			os.Exit(1)
		}
		return c.updateStage(oldStage, newStage)
	}
	return fmt.Errorf("stage %v is not added to scheduler cache, so cannot be updated", key)
}

func (c *cacheImpl) updateStage(oldStage, newStage *meta.Stage) error {
	if err := c.removeStage(oldStage); err != nil {
		return err
	}
	return c.addStage(newStage, false)
}

func (c *cacheImpl) addStage(stage *meta.Stage, assumeStage bool) error {
	key, err := framework.GetStageKey(stage)
	if err != nil {
		return err
	}
	n, ok := c.workers[stage.WorkerUID]
	if !ok {
		n = newWorkerInfoListItem(framework.NewWorkerInfo())
		c.workers[stage.WorkerUID] = n
	}
	n.info.AddStage(stage)
	c.moveWorkerInfoToHead(stage.WorkerUID)
	ps := &stageState{
		stage: stage,
	}
	c.stageStates[key] = ps
	if assumeStage {
		c.assumedStages[key] = struct{}{}
	}
	return nil
}

func (c *cacheImpl) RemoveStage(stage *meta.Stage) error {
	key, err := framework.GetStageKey(stage)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	currState, ok := c.stageStates[key]
	if !ok {
		return fmt.Errorf("stage %v is not found in scheduler cache, so cannot be removed from it", key)
	}
	if currState.stage.WorkerUID != stage.WorkerUID {
		flog.Errorf("Stage was added to a different worker than it was assumed %s %s", stage.WorkerUID, currState.stage.WorkerUID)
		if stage.WorkerUID != "" {
			// An empty WorkerName is possible when the scheduler misses a Delete
			// event and it gets the last known state from the informer cache.
			flog.Errorf("scheduler cache is corrupted and can badly affect scheduling decisions")
			os.Exit(1)
		}
	}
	return c.removeStage(currState.stage)
}

func (c *cacheImpl) removeStage(stage *meta.Stage) error {
	key, err := framework.GetStageKey(stage)
	if err != nil {
		return err
	}

	n, ok := c.workers[stage.WorkerUID]
	if !ok {
		flog.Errorf("Worker not found when trying to remove stage, %v", stage)
	} else {
		if err := n.info.RemoveStage(stage); err != nil {
			return err
		}
		if len(n.info.Stages) == 0 && n.info.Worker() == nil {
			c.removeWorkerInfoFromList(stage.WorkerUID)
		} else {
			c.moveWorkerInfoToHead(stage.WorkerUID)
		}
	}

	delete(c.stageStates, key)
	delete(c.assumedStages, key)
	return nil
}

func (c *cacheImpl) GetStage(stage *meta.Stage) (*meta.Stage, error) {
	key, err := framework.GetStageKey(stage)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	stageState, ok := c.stageStates[key]
	if !ok {
		return nil, fmt.Errorf("stage %v does not exist in scheduler cache", key)
	}

	return stageState.stage, nil
}

func (c *cacheImpl) IsAssumedStage(stage *meta.Stage) (bool, error) {
	key, err := framework.GetStageKey(stage)
	if err != nil {
		return false, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	_, ok := c.assumedStages[key]
	return ok, nil
}

func (c *cacheImpl) AddWorker(worker *meta.Worker) *framework.WorkerInfo {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.workers[worker.UID]
	if !ok {
		n = newWorkerInfoListItem(framework.NewWorkerInfo())
		c.workers[worker.UID] = n
	}
	c.moveWorkerInfoToHead(worker.UID)

	c.workerTree.addWorker(worker)
	n.info.SetWorker(worker)
	return n.info.Clone()
}

func (c *cacheImpl) UpdateWorker(oldWorker, newWorker *meta.Worker) *framework.WorkerInfo {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.workers[newWorker.UID]
	if !ok {
		n = newWorkerInfoListItem(framework.NewWorkerInfo())
		c.workers[newWorker.UID] = n
		c.workerTree.addWorker(newWorker)
	}
	c.moveWorkerInfoToHead(newWorker.UID)

	c.workerTree.updateWorker(oldWorker, newWorker)
	n.info.SetWorker(newWorker)
	return n.info.Clone()
}

func newWorkerInfoListItem(ni *framework.WorkerInfo) *workerInfoListItem {
	return &workerInfoListItem{
		info: ni,
	}
}

func (c *cacheImpl) RemoveWorker(worker *meta.Worker) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.workers[worker.UID]
	if !ok {
		return fmt.Errorf("worker %v is not found", worker.UID)
	}
	n.info.RemoveWorker()

	if len(n.info.Stages) == 0 {
		c.removeWorkerInfoFromList(worker.UID)
	} else {
		c.moveWorkerInfoToHead(worker.UID)
	}
	if err := c.workerTree.removeWorker(worker); err != nil {
		return err
	}
	return nil
}

func (c *cacheImpl) removeWorkerInfoFromList(name string) {
	ni, ok := c.workers[name]
	if !ok {
		flog.Errorf("No worker info with given name found in the cache, %s", name)
		return
	}

	if ni.prev != nil {
		ni.prev.next = ni.next
	}
	if ni.next != nil {
		ni.next.prev = ni.prev
	}
	// if the removed item was at the head, we must update the head.
	if ni == c.headWorker {
		c.headWorker = ni.next
	}
	delete(c.workers, name)
}

func (c *cacheImpl) moveWorkerInfoToHead(name string) {
	ni, ok := c.workers[name]
	if !ok {
		flog.Errorf("No worker info with given name found in the cache, %s", name)
		return
	}
	// if the worker info list item is already at the head, we are done.
	if ni == c.headWorker {
		return
	}

	if ni.prev != nil {
		ni.prev.next = ni.next
	}
	if ni.next != nil {
		ni.next.prev = ni.prev
	}
	if c.headWorker != nil {
		c.headWorker.prev = ni
	}
	ni.next = c.headWorker
	ni.prev = nil
	c.headWorker = ni
}

func (c *cacheImpl) UpdateSnapshot(workerSnapshot *Snapshot) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Get the last generation of the snapshot.
	snapshotGeneration := workerSnapshot.generation

	// WorkerInfoList and HaveStagesWithAffinityWorkerInfoList must be re-created if a worker was added
	// or removed from the cache.
	updateAllLists := false

	// Start from the head of the WorkerInfo doubly linked list and update snapshot
	// of WorkerInfos updated after the last snapshot.
	for worker := c.headWorker; worker != nil; worker = worker.next {
		if worker.info.Generation <= snapshotGeneration {
			// all the workers are updated before the existing snapshot. We are done.
			break
		}
		if np := worker.info.Worker(); np != nil {
			existing, ok := workerSnapshot.workerInfoMap[np.Name]
			if !ok {
				updateAllLists = true
				existing = &framework.WorkerInfo{}
				workerSnapshot.workerInfoMap[np.Name] = existing
			}
			clone := worker.info.Clone()
			// We need to preserve the original pointer of the WorkerInfo struct since it
			// is used in the WorkerInfoList, which we may not update.
			*existing = *clone
		}
	}
	// Update the snapshot generation with the latest WorkerInfo generation.
	if c.headWorker != nil {
		workerSnapshot.generation = c.headWorker.info.Generation
	}

	// Comparing to stages in workerTree.
	// Deleted workers get removed from the tree, but they might remain in the workers map
	// if they still have non-deleted Stages.
	if len(workerSnapshot.workerInfoMap) > c.workerTree.numWorkers {
		c.removeDeletedWorkersFromSnapshot(workerSnapshot)
		updateAllLists = true
	}

	if updateAllLists {
		c.updateWorkerInfoSnapshotList(workerSnapshot, updateAllLists)
	}

	if len(workerSnapshot.workerInfoList) != c.workerTree.numWorkers {
		errMsg := fmt.Sprintf("snapshot state is not consistent, length of WorkerInfoList=%v not equal to length of workers in tree=%v "+
			", length of WorkerInfoMap=%v, length of workers in cache=%v"+
			", trying to recover",
			len(workerSnapshot.workerInfoList), c.workerTree.numWorkers,
			len(workerSnapshot.workerInfoMap), len(c.workers))
		flog.Errorf(errMsg)
		// We will try to recover by re-creating the lists for the next scheduling cycle, but still return an
		// error to surface the problem, the error will likely cause a failure to the current scheduling cycle.
		c.updateWorkerInfoSnapshotList(workerSnapshot, true)
		return fmt.Errorf(errMsg)
	}

	return nil
}

func (c *cacheImpl) removeDeletedWorkersFromSnapshot(snapshot *Snapshot) {
	toDelete := len(snapshot.workerInfoMap) - c.workerTree.numWorkers
	for name := range snapshot.workerInfoMap {
		if toDelete <= 0 {
			break
		}
		if n, ok := c.workers[name]; !ok || n.info.Worker() == nil {
			delete(snapshot.workerInfoMap, name)
			toDelete--
		}
	}
}

func (c *cacheImpl) updateWorkerInfoSnapshotList(snapshot *Snapshot, updateAll bool) {
	if updateAll {
		// Take a snapshot of the workers order in the tree
		snapshot.workerInfoList = make([]*framework.WorkerInfo, 0, c.workerTree.numWorkers)
		workersList, err := c.workerTree.list()
		if err != nil {
			flog.Error(err)
			flog.Errorf("Error occurred while retrieving the list of names of the workers from worker tree")
		}
		for _, workerName := range workersList {
			if workerInfo := snapshot.workerInfoMap[workerName]; workerInfo != nil {
				snapshot.workerInfoList = append(snapshot.workerInfoList, workerInfo)
			} else {
				flog.Errorf("Worker exists in workerTree but not in WorkerInfoMap, this should not happen, %s", workerName)
			}
		}
	}
}

func (c *cacheImpl) Dump() *Dump {
	c.mu.RLock()
	defer c.mu.RUnlock()

	workers := make(map[string]*framework.WorkerInfo, len(c.workers))
	for k, v := range c.workers {
		workers[k] = v.info.Clone()
	}

	return &Dump{
		Workers:       workers,
		AssumedStages: Union(c.assumedStages, nil),
	}
}

func Union(s1, s2 map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	for key := range s1 {
		result[key] = struct{}{}
	}
	for key := range s2 {
		result[key] = struct{}{}
	}
	return result
}

func (c *cacheImpl) run() {
	go parallelizer.JitterUntil(c.cleanupExpiredAssumedStages, c.period, 0.0, true, c.stop)
}

func (c *cacheImpl) cleanupExpiredAssumedStages() {
	c.cleanupAssumedStages(time.Now())
}

func (c *cacheImpl) cleanupAssumedStages(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.assumedStages {
		ps, ok := c.stageStates[key]
		if !ok {
			flog.Errorf("Key found in assumed set but not in stageStates, potentially a logical error")
			os.Exit(1)
		}
		if !ps.bindingFinished {
			flog.Info("Could not expire cache for stage as binding is still in progress")
			continue
		}
		if c.ttl != 0 && now.After(*ps.deadline) {
			flog.Infof("stage expired %v", ps.stage)
			if err := c.RemoveStage(ps.stage); err != nil {
				flog.Errorf("expire stage failed %s %v", err, ps.stage)
			}
		}
	}
}

type stageState struct {
	stage *meta.Stage
	// Used by assumedStage to determinate expiration.
	// If deadline is nil, assumedStage will never expire.
	deadline *time.Time
	// Used to block cache from expiring assumedStage if binding still runs
	bindingFinished bool
}

type workerInfoListItem struct {
	info *framework.WorkerInfo
	next *workerInfoListItem
	prev *workerInfoListItem
}
