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

		workers:     make(map[string]*workerInfoListItem),
		workerTree:  newWorkerTree(nil),
		assumedPods: make(map[string]struct{}),
		stageStates: make(map[string]*stageState),
		imageStates: make(map[string]*imageState),
	}
}

type cacheImpl struct {
	stop   <-chan struct{}
	ttl    time.Duration
	period time.Duration

	// This mutex guards all fields within this cache struct.
	mu sync.RWMutex
	// a set of assumed pod keys.
	// The key could further be used to get an entry in podStates.
	assumedPods map[string]struct{}
	// a map from pod key to podState.
	stageStates map[string]*stageState
	workers     map[string]*workerInfoListItem
	// headNode points to the most recently updated NodeInfo in "nodes". It is the
	// head of the linked list.
	headWorker *workerInfoListItem
	workerTree *workerTree
	// A map from image name to its imageState.
	imageStates map[string]*imageState
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
	key, err := framework.GetPodKey(stage)
	if err != nil {
		return err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if _, ok := c.stageStates[key]; ok {
		return fmt.Errorf("pod %v is in the cache, so can't be assumed", key)
	}

	return c.addStage(stage, true)
}

func (c *cacheImpl) FinishBinding(stage *meta.Stage) error {
	return c.finishBinding(stage, time.Now())
}

func (c *cacheImpl) finishBinding(stage *meta.Stage, now time.Time) error {
	key, err := framework.GetPodKey(stage)
	if err != nil {
		return err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	flog.Infof("Finished binding for pod, can be expired, %s %s, worker %s %s", stage.Name, stage.UID, stage.WorkerUID, stage.WorkerHost)
	currState, ok := c.stageStates[key]
	_, has := c.assumedPods[key]
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
	key, err := framework.GetPodKey(stage)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	currState, ok := c.stageStates[key]
	if ok && currState.stage.WorkerUID != stage.WorkerUID {
		return fmt.Errorf("pod %v was assumed on %v but assigned to %v", key, stage.WorkerUID, currState.stage.WorkerUID)
	}

	// Only assumed pod can be forgotten.
	_, has := c.assumedPods[key]
	if ok && has {
		return c.removeStage(stage)
	}
	return fmt.Errorf("pod %v wasn't assumed so cannot be forgotten", key)
}

func (c *cacheImpl) AddStage(stage *meta.Stage) error {
	key, err := framework.GetPodKey(stage)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	currState, ok := c.stageStates[key]
	_, has := c.assumedPods[key]
	switch {
	case ok && has:
		if currState.stage.WorkerUID != stage.WorkerUID {
			// The pod was added to a different node than it was assumed to.
			flog.Infof("Pod was added to a different node than it was assumed, %v", stage)
			if err = c.updateStage(currState.stage, stage); err != nil {
				flog.Errorf("%s Error occurred while updating pod", err)
			}
		} else {
			delete(c.assumedPods, key)
			c.stageStates[key].deadline = nil
			c.stageStates[key].stage = stage
		}
	case !ok:
		// Pod was expired. We should add it back.
		if err = c.addStage(stage, false); err != nil {
			flog.Errorf("Error occurred while adding pod %s", err)
		}
	default:
		return fmt.Errorf("pod %v was already in added state", key)
	}
	return nil
}

func (c *cacheImpl) UpdateStage(oldStage, newStage *meta.Stage) error {
	key, err := framework.GetPodKey(oldStage)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	currState, ok := c.stageStates[key]
	// An assumed pod won't have Update/Remove event. It needs to have Add event
	// before Update event, in which case the state would change from Assumed to Added.
	_, has := c.assumedPods[key]
	if ok && !has {
		if currState.stage.WorkerUID != newStage.WorkerUID {
			flog.Errorf("Pod updated on a different node than previously added to %v", oldStage)
			flog.Errorf("scheduler cache is corrupted and can badly affect scheduling decisions")
			os.Exit(1)
		}
		return c.updateStage(oldStage, newStage)
	}
	return fmt.Errorf("pod %v is not added to scheduler cache, so cannot be updated", key)
}

func (c *cacheImpl) updateStage(oldStage, newStage *meta.Stage) error {
	if err := c.removeStage(oldStage); err != nil {
		return err
	}
	return c.addStage(newStage, false)
}

func (c *cacheImpl) addStage(stage *meta.Stage, assumePod bool) error {
	key, err := framework.GetPodKey(stage)
	if err != nil {
		return err
	}
	n, ok := c.workers[stage.WorkerUID]
	if !ok {
		n = newWorkerInfoListItem(framework.NewWorkerInfo())
		c.workers[stage.WorkerUID] = n
	}
	n.info.AddStage(stage)
	c.moveNodeInfoToHead(stage.WorkerUID)
	ps := &stageState{
		stage: stage,
	}
	c.stageStates[key] = ps
	if assumePod {
		c.assumedPods[key] = struct{}{}
	}
	return nil
}

func (c *cacheImpl) RemoveStage(stage *meta.Stage) error {
	key, err := framework.GetPodKey(stage)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	currState, ok := c.stageStates[key]
	if !ok {
		return fmt.Errorf("pod %v is not found in scheduler cache, so cannot be removed from it", key)
	}
	if currState.stage.WorkerUID != stage.WorkerUID {
		flog.Errorf("Pod was added to a different node than it was assumed %s %s", stage.WorkerUID, currState.stage.WorkerUID)
		if stage.WorkerUID != "" {
			// An empty NodeName is possible when the scheduler misses a Delete
			// event and it gets the last known state from the informer cache.
			flog.Errorf("scheduler cache is corrupted and can badly affect scheduling decisions")
			os.Exit(1)
		}
	}
	return c.removeStage(currState.stage)
}

func (c *cacheImpl) removeStage(stage *meta.Stage) error {
	key, err := framework.GetPodKey(stage)
	if err != nil {
		return err
	}

	n, ok := c.workers[stage.WorkerUID]
	if !ok {
		flog.Errorf("Node not found when trying to remove pod, %v", stage)
	} else {
		if err := n.info.RemoveStage(stage); err != nil {
			return err
		}
		if len(n.info.Stages) == 0 && n.info.Worker() == nil {
			c.removeNodeInfoFromList(stage.WorkerUID)
		} else {
			c.moveNodeInfoToHead(stage.WorkerUID)
		}
	}

	delete(c.stageStates, key)
	delete(c.assumedPods, key)
	return nil
}

func (c *cacheImpl) GetStage(stage *meta.Stage) (*meta.Stage, error) {
	key, err := framework.GetPodKey(stage)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	podState, ok := c.stageStates[key]
	if !ok {
		return nil, fmt.Errorf("stage %v does not exist in scheduler cache", key)
	}

	return podState.stage, nil
}

func (c *cacheImpl) IsAssumedStage(stage *meta.Stage) (bool, error) {
	key, err := framework.GetPodKey(stage)
	if err != nil {
		return false, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	_, ok := c.assumedPods[key]
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
	c.moveNodeInfoToHead(worker.UID)

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
	c.moveNodeInfoToHead(newWorker.UID)

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
		c.removeNodeInfoFromList(worker.UID)
	} else {
		c.moveNodeInfoToHead(worker.UID)
	}
	if err := c.workerTree.removeWorker(worker); err != nil {
		return err
	}
	return nil
}

func (c *cacheImpl) removeNodeInfoFromList(name string) {
	ni, ok := c.workers[name]
	if !ok {
		flog.Errorf("No node info with given name found in the cache, %s", name)
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

func (c *cacheImpl) moveNodeInfoToHead(name string) {
	ni, ok := c.workers[name]
	if !ok {
		flog.Errorf("No node info with given name found in the cache, %s", name)
		return
	}
	// if the node info list item is already at the head, we are done.
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

	// NodeInfoList and HavePodsWithAffinityNodeInfoList must be re-created if a node was added
	// or removed from the cache.
	updateAllLists := false

	// Start from the head of the NodeInfo doubly linked list and update snapshot
	// of NodeInfos updated after the last snapshot.
	for node := c.headWorker; node != nil; node = node.next {
		if node.info.Generation <= snapshotGeneration {
			// all the nodes are updated before the existing snapshot. We are done.
			break
		}
		if np := node.info.Worker(); np != nil {
			existing, ok := workerSnapshot.nodeInfoMap[np.Name]
			if !ok {
				updateAllLists = true
				existing = &framework.WorkerInfo{}
				workerSnapshot.nodeInfoMap[np.Name] = existing
			}
			clone := node.info.Clone()
			// We need to preserve the original pointer of the NodeInfo struct since it
			// is used in the NodeInfoList, which we may not update.
			*existing = *clone
		}
	}
	// Update the snapshot generation with the latest NodeInfo generation.
	if c.headWorker != nil {
		workerSnapshot.generation = c.headWorker.info.Generation
	}

	// Comparing to pods in nodeTree.
	// Deleted nodes get removed from the tree, but they might remain in the nodes map
	// if they still have non-deleted Pods.
	if len(workerSnapshot.nodeInfoMap) > c.workerTree.numNodes {
		//c.removeDeletedWorkersFromSnapshot(workerSnapshot) fixme
		updateAllLists = true
	}

	if updateAllLists {
		//c.updateWorkerInfoSnapshotList(workerSnapshot, updateAllLists) fixme
		return nil // fixme
	}

	if len(workerSnapshot.nodeInfoList) != c.workerTree.numNodes {
		errMsg := fmt.Sprintf("snapshot state is not consistent, length of NodeInfoList=%v not equal to length of nodes in tree=%v "+
			", length of NodeInfoMap=%v, length of nodes in cache=%v"+
			", trying to recover",
			len(workerSnapshot.nodeInfoList), c.workerTree.numNodes,
			len(workerSnapshot.nodeInfoMap), len(c.workers))
		flog.Errorf(errMsg)
		// We will try to recover by re-creating the lists for the next scheduling cycle, but still return an
		// error to surface the problem, the error will likely cause a failure to the current scheduling cycle.
		c.updateWorkerInfoSnapshotList(workerSnapshot, true)
		return fmt.Errorf(errMsg)
	}

	return nil
}

func (c *cacheImpl) removeDeletedWorkersFromSnapshot(snapshot *Snapshot) {
	toDelete := len(snapshot.nodeInfoMap) - c.workerTree.numNodes
	for name := range snapshot.nodeInfoMap {
		if toDelete <= 0 {
			break
		}
		if n, ok := c.workers[name]; !ok || n.info.Worker() == nil {
			delete(snapshot.nodeInfoMap, name)
			toDelete--
		}
	}
}

func (c *cacheImpl) updateWorkerInfoSnapshotList(snapshot *Snapshot, updateAll bool) {
	if updateAll {
		// Take a snapshot of the nodes order in the tree
		snapshot.nodeInfoList = make([]*framework.WorkerInfo, 0, c.workerTree.numNodes)
		nodesList, err := c.workerTree.list()
		if err != nil {
			flog.Error(err)
			flog.Errorf("Error occurred while retrieving the list of names of the nodes from node tree")
		}
		for _, nodeName := range nodesList {
			if nodeInfo := snapshot.nodeInfoMap[nodeName]; nodeInfo != nil {
				snapshot.nodeInfoList = append(snapshot.nodeInfoList, nodeInfo)
			} else {
				flog.Errorf("Node exists in nodeTree but not in NodeInfoMap, this should not happen, %s", nodeName)
			}
		}
	}
}

func (c *cacheImpl) Dump() *Dump {
	c.mu.RLock()
	defer c.mu.RUnlock()

	nodes := make(map[string]*framework.WorkerInfo, len(c.workers))
	for k, v := range c.workers {
		nodes[k] = v.info.Clone()
	}

	return &Dump{
		Workers:       nodes,
		AssumedStages: Union(c.assumedPods, nil),
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

	for key := range c.assumedPods {
		ps, ok := c.stageStates[key]
		if !ok {
			flog.Errorf("Key found in assumed set but not in podStates, potentially a logical error")
			os.Exit(1)
		}
		if !ps.bindingFinished {
			flog.Info("Could not expire cache for pod as binding is still in progress")
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
	// Used by assumedPod to determinate expiration.
	// If deadline is nil, assumedPod will never expire.
	deadline *time.Time
	// Used to block cache from expiring assumedPod if binding still runs
	bindingFinished bool
}

type workerInfoListItem struct {
	info *framework.WorkerInfo
	next *workerInfoListItem
	prev *workerInfoListItem
}

type imageState struct {
	// Size of the image
	size int64
	// A set of node names for nodes having this image present
	nodes map[string]struct{}
}
