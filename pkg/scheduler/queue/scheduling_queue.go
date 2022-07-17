package queue

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/scheduler/heap"
	"github.com/tsundata/flowline/pkg/util/clock"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"reflect"
	"sync"
	"time"
)

var (
	// AssignedStageAdd is the event when a pod is added that causes pods with matching affinity terms
	// to be more schedulable.
	AssignedStageAdd = framework.ClusterEvent{Resource: framework.Stage, ActionType: framework.Add, Label: "AssignedPodAdd"}
	// WorkerAdd is the event when a new node is added to the cluster.
	WorkerAdd = framework.ClusterEvent{Resource: framework.Worker, ActionType: framework.Add, Label: "NodeAdd"}
	// AssignedStageUpdate is the event when a pod is updated that causes pods with matching affinity
	// terms to be more schedulable.
	AssignedStageUpdate = framework.ClusterEvent{Resource: framework.Stage, ActionType: framework.Update, Label: "AssignedPodUpdate"}
	// AssignedPodDelete is the event when a pod is deleted that causes pods with matching affinity
	// terms to be more schedulable.
	AssignedPodDelete = framework.ClusterEvent{Resource: framework.Stage, ActionType: framework.Delete, Label: "AssignedPodDelete"}
	// UnschedulableTimeout is the event when a pod stays in unschedulable for longer than timeout.
	UnschedulableTimeout = framework.ClusterEvent{Resource: framework.WildCard, ActionType: framework.All, Label: "UnschedulableTimeout"}
)

// PreEnqueueCheck is a function type. It's used to build functions that
// run against a Pod and the caller can choose to enqueue or skip the Pod
// by the checking result.
type PreEnqueueCheck func(stage *meta.Stage) bool

type SchedulingQueue interface {
	framework.StageNominator
	Add(stage *meta.Stage) error
	// Activate moves the given pods to activeQ iff they're in unschedulablePods or backoffQ.
	// The passed-in pods are originally compiled from plugins that want to activate Pods,
	// by injecting the pods through a reserved CycleState struct (PodsToActivate).
	Activate(stages map[string]*meta.Stage)
	// AddUnschedulableIfNotPresent adds an unschedulable pod back to scheduling queue.
	// The podSchedulingCycle represents the current scheduling cycle number which can be
	// returned by calling SchedulingCycle().
	AddUnschedulableIfNotPresent(stage *framework.QueuedStageInfo, stageSchedulingCycle int64) error
	// SchedulingCycle returns the current number of scheduling cycle which is
	// cached by scheduling queue. Normally, incrementing this number whenever
	// a pod is popped (e.g. called Pop()) is enough.
	SchedulingCycle() int64
	// Pop removes the head of the queue and returns it. It blocks if the
	// queue is empty and waits until a new item is added to the queue.
	Pop() (*framework.QueuedStageInfo, error)
	Update(oldStage, newStage *meta.Stage) error
	Delete(stage *meta.Stage) error
	MoveAllToActiveOrBackoffQueue(event framework.ClusterEvent, preCheck PreEnqueueCheck)
	AssignedPodAdded(stage *meta.Stage)
	AssignedPodUpdated(stage *meta.Stage)
	PendingPods() []*meta.Stage
	// Close closes the SchedulingQueue so that the goroutine which is
	// waiting to pop items can exit gracefully.
	Close()
	// Run starts the goroutines managing the queue.
	Run()
}

type priorityQueueOptions struct {
	clock                             clock.Clock
	podInitialBackoffDuration         time.Duration
	podMaxBackoffDuration             time.Duration
	podMaxInUnschedulablePodsDuration time.Duration
	podNominator                      framework.StageNominator
	clusterEventMap                   map[framework.ClusterEvent]map[string]struct{}
}

const (
	// DefaultPodMaxInUnschedulablePodsDuration is the default value for the maximum
	// time a pod can stay in unschedulablePods. If a pod stays in unschedulablePods
	// for longer than this value, the pod will be moved from unschedulablePods to
	// backoffQ or activeQ. If this value is empty, the default value (5min)
	// will be used.
	DefaultPodMaxInUnschedulablePodsDuration time.Duration = 5 * time.Minute

	queueClosed = "scheduling queue is closed"
)

const (
	// DefaultPodInitialBackoffDuration is the default value for the initial backoff duration
	// for unschedulable pods. To change the default podInitialBackoffDurationSeconds used by the
	// scheduler, update the ComponentConfig value in defaults.go
	DefaultPodInitialBackoffDuration time.Duration = 1 * time.Second
	// DefaultPodMaxBackoffDuration is the default value for the max backoff duration
	// for unschedulable pods. To change the default podMaxBackoffDurationSeconds used by the
	// scheduler, update the ComponentConfig value in defaults.go
	DefaultPodMaxBackoffDuration time.Duration = 10 * time.Second
)

var defaultPriorityQueueOptions = priorityQueueOptions{
	clock:                             clock.RealClock{},
	podInitialBackoffDuration:         DefaultPodInitialBackoffDuration,
	podMaxBackoffDuration:             DefaultPodMaxBackoffDuration,
	podMaxInUnschedulablePodsDuration: DefaultPodMaxInUnschedulablePodsDuration,
}

// Option configures a PriorityQueue
type Option func(*priorityQueueOptions)

// NewSchedulingQueue initializes a priority queue as a new scheduling queue.
func NewSchedulingQueue(
	lessFn framework.LessFunc,
	informerFactory interface{},
	opts ...Option) SchedulingQueue {
	return NewPriorityQueue(lessFn, informerFactory, opts...)
}

func MakeNextPodFunc(queue SchedulingQueue) func() *framework.QueuedStageInfo {
	return func() *framework.QueuedStageInfo {
		stageInfo, err := queue.Pop()
		if err == nil {
			flog.Infof("About to try and schedule stage %s %s", stageInfo.Stage.Name, stageInfo.Stage.UID)
			return stageInfo
		}
		flog.Errorf("%s Error while retrieving next stage from scheduling queue", err)
		return nil
	}
}

// newQueuedStageInfoForLookup builds a QueuedPodInfo object for a lookup in the queue.
func newQueuedStageInfoForLookup(stage *meta.Stage, plugins ...string) *framework.QueuedStageInfo {
	sets := make(map[string]struct{})
	for _, plugin := range plugins {
		sets[plugin] = struct{}{}
	}
	// Since this is only used for a lookup in the queue, we only need to set the Pod,
	// and so we avoid creating a full PodInfo, which is expensive to instantiate frequently.
	return &framework.QueuedStageInfo{
		StageInfo:            &framework.StageInfo{Stage: stage},
		UnschedulablePlugins: sets,
	}
}

// PriorityQueue implements a scheduling queue.
// The head of PriorityQueue is the highest priority pending pod. This structure
// has two sub queues and a additional data structure, namely: activeQ,
// backoffQ and unschedulablePods.
// - activeQ holds pods that are being considered for scheduling.
// - backoffQ holds pods that moved from unschedulablePods and will move to
//   activeQ when their backoff periods complete.
// - unschedulablePods holds pods that were already attempted for scheduling and
//   are currently determined to be unschedulable.
type PriorityQueue struct {
	// PodNominator abstracts the operations to maintain nominated Pods.
	framework.StageNominator

	stop  chan struct{}
	clock clock.Clock

	// pod initial backoff duration.
	podInitialBackoffDuration time.Duration
	// pod maximum backoff duration.
	podMaxBackoffDuration time.Duration
	// the maximum time a pod can stay in the unschedulablePods.
	podMaxInUnschedulablePodsDuration time.Duration

	lock sync.RWMutex
	cond sync.Cond

	// activeQ is heap structure that scheduler actively looks at to find pods to
	// schedule. Head of heap is the highest priority pod.
	activeQ *heap.Heap
	// podBackoffQ is a heap ordered by backoff expiry. Pods which have completed backoff
	// are popped from this heap before the scheduler looks at activeQ
	podBackoffQ *heap.Heap
	// unschedulablePods holds pods that have been tried and determined unschedulable.
	unschedulableStages *UnschedulableStages
	// schedulingCycle represents sequence number of scheduling cycle and is incremented
	// when a pod is popped.
	schedulingCycle int64
	// moveRequestCycle caches the sequence number of scheduling cycle when we
	// received a move request. Unschedulable pods in and before this scheduling
	// cycle will be put back to activeQueue if we were trying to schedule them
	// when we received move request.
	moveRequestCycle int64

	clusterEventMap map[framework.ClusterEvent]map[string]struct{}

	// closed indicates that the queue is closed.
	// It is mainly used to let Pop() exit its control loop while waiting for an item.
	closed bool

	nsLister interface{}
}

// newQueuedPodInfo builds a QueuedPodInfo object.
func (p *PriorityQueue) newQueuedStageInfo(stage *meta.Stage, plugins ...string) *framework.QueuedStageInfo {
	sets := make(map[string]struct{})
	for _, plugin := range plugins {
		sets[plugin] = struct{}{}
	}
	now := p.clock.Now()
	return &framework.QueuedStageInfo{
		StageInfo:               framework.NewStageInfo(stage),
		Timestamp:               now,
		InitialAttemptTimestamp: now,
		UnschedulablePlugins:    sets,
	}
}

func (p *PriorityQueue) activate(stage *meta.Stage) bool {
	// Verify if the pod is present in activeQ.
	if _, exists, _ := p.activeQ.Get(newQueuedStageInfoForLookup(stage)); exists {
		// No need to activate if it's already present in activeQ.
		return false
	}
	var pInfo *framework.QueuedStageInfo
	// Verify if the pod is present in unschedulablePods or backoffQ.
	if pInfo = p.unschedulableStages.get(stage); pInfo == nil {
		// If the pod doesn't belong to unschedulablePods or backoffQ, don't activate it.
		if obj, exists, _ := p.podBackoffQ.Get(newQueuedStageInfoForLookup(stage)); !exists {
			flog.Errorf("To-activate pod does not exist in unschedulablePods or backoffQ, %v", stage)
			return false
		} else {
			pInfo = obj.(*framework.QueuedStageInfo)
		}
	}

	if pInfo == nil {
		// Redundant safe check. We shouldn't reach here.
		flog.Errorf("Internal error: cannot obtain pInfo")
		return false
	}

	if err := p.activeQ.Add(pInfo); err != nil {
		flog.Errorf("Error adding pod to the scheduling queue, %v", stage)
		return false
	}
	p.unschedulableStages.delete(stage)
	p.podBackoffQ.Delete(pInfo)
	p.StageNominator.AddNominatedStage(pInfo.StageInfo, nil)
	return true
}

func (p *PriorityQueue) Add(stage *meta.Stage) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	pInfo := p.newQueuedStageInfo(stage)
	if err := p.activeQ.Add(pInfo); err != nil {
		flog.Errorf("Error adding stage to the active queue, %v", stage)
		return err
	}
	if p.unschedulableStages.get(stage) != nil {
		flog.Errorf("Error: stage is already in the unschedulable queue, %v", stage)
		p.unschedulableStages.delete(stage)
	}
	// Delete pod from backoffQ if it is backing off
	if err := p.podBackoffQ.Delete(pInfo); err == nil {
		flog.Errorf("Error: stage is already in the podBackoff queue, %v", stage)
	}
	p.StageNominator.AddNominatedStage(pInfo.StageInfo, nil)
	p.cond.Broadcast()

	return nil
}

func (p *PriorityQueue) Activate(stages map[string]*meta.Stage) {
	p.lock.Lock()
	defer p.lock.Unlock()

	activated := false
	for _, pod := range stages {
		if p.activate(pod) {
			activated = true
		}
	}

	if activated {
		p.cond.Broadcast()
	}
}

func (p *PriorityQueue) AddUnschedulableIfNotPresent(pInfo *framework.QueuedStageInfo, stageSchedulingCycle int64) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	stage := pInfo.Stage
	if p.unschedulableStages.get(stage) != nil {
		return fmt.Errorf("stage %v %s is already present in unschedulable queue", stage.Name, stage.UID)
	}

	if _, exists, _ := p.activeQ.Get(pInfo); exists {
		return fmt.Errorf("stage %v is already present in the active queue", stage)
	}
	if _, exists, _ := p.podBackoffQ.Get(pInfo); exists {
		return fmt.Errorf("stage %v is already present in the backoff queue", stage)
	}

	// Refresh the timestamp since the pod is re-added.
	pInfo.Timestamp = p.clock.Now()

	// If a move request has been received, move it to the BackoffQ, otherwise move
	// it to unschedulableStages.
	if p.moveRequestCycle >= stageSchedulingCycle {
		if err := p.podBackoffQ.Add(pInfo); err != nil {
			return fmt.Errorf("error adding stage %v to the backoff queue: %v", stage.Name, err)
		}
	} else {
		p.unschedulableStages.addOrUpdate(pInfo)
	}

	p.StageNominator.AddNominatedStage(pInfo.StageInfo, nil)
	return nil
}

func (p *PriorityQueue) SchedulingCycle() int64 {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.schedulingCycle
}

func (p *PriorityQueue) Pop() (*framework.QueuedStageInfo, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	for p.activeQ.Len() == 0 {
		// When the queue is empty, invocation of Pop() is blocked until new item is enqueued.
		// When Close() is called, the p.closed is set and the condition is broadcast,
		// which causes this loop to continue and return from the Pop().
		if p.closed {
			return nil, fmt.Errorf(queueClosed)
		}
		p.cond.Wait()
	}
	obj, err := p.activeQ.Pop()
	if err != nil {
		return nil, err
	}
	pInfo := obj.(*framework.QueuedStageInfo)
	pInfo.Attempts++
	p.schedulingCycle++
	return pInfo, nil
}

func updateStage(oldPodInfo interface{}, newStage *meta.Stage) *framework.QueuedStageInfo {
	pInfo := oldPodInfo.(*framework.QueuedStageInfo)
	pInfo.Update(newStage)
	return pInfo
}

// isPodUpdated checks if the pod is updated in a way that it may have become
// schedulable. It drops status of the pod and compares it with old version.
func isStageUpdated(oldStage, newStage *meta.Stage) bool {
	strip := func(stage *meta.Stage) *meta.Stage {
		p := stage // todo deep copy
		p.ResourceVersion = ""
		p.Generation = 0
		//p.Status = v1.PodStatus{}
		//p.ManagedFields = nil
		p.Finalizers = nil
		return p
	}
	return !reflect.DeepEqual(strip(oldStage), strip(newStage))
}

// isStageBackingoff returns true if a pod is still waiting for its backoff timer.
// If this returns true, the pod should not be re-tried.
func (p *PriorityQueue) isStageBackingoff(podInfo *framework.QueuedStageInfo) bool {
	boTime := p.getBackoffTime(podInfo)
	return boTime.After(p.clock.Now())
}

func (p *PriorityQueue) Update(oldStage, newStage *meta.Stage) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if oldStage != nil {
		oldPodInfo := newQueuedStageInfoForLookup(oldStage)
		// If the pod is already in the active queue, just update it there.
		if oldPodInfo, exists, _ := p.activeQ.Get(oldPodInfo); exists {
			pInfo := updateStage(oldPodInfo, newStage)
			p.StageNominator.UpdateNominatedStage(oldStage, pInfo.StageInfo)
			return p.activeQ.Update(pInfo)
		}

		// If the pod is in the backoff queue, update it there.
		if oldPodInfo, exists, _ := p.podBackoffQ.Get(oldPodInfo); exists {
			pInfo := updateStage(oldPodInfo, newStage)
			p.StageNominator.UpdateNominatedStage(oldStage, pInfo.StageInfo)
			return p.podBackoffQ.Update(pInfo)
		}
	}

	// If the pod is in the unschedulable queue, updating it may make it schedulable.
	if usPodInfo := p.unschedulableStages.get(newStage); usPodInfo != nil {
		pInfo := updateStage(usPodInfo, newStage)
		p.StageNominator.UpdateNominatedStage(oldStage, pInfo.StageInfo)
		if isStageUpdated(oldStage, newStage) {
			if p.isStageBackingoff(usPodInfo) {
				if err := p.podBackoffQ.Add(pInfo); err != nil {
					return err
				}
				p.unschedulableStages.delete(usPodInfo.Stage)
			} else {
				if err := p.activeQ.Add(pInfo); err != nil {
					return err
				}
				p.unschedulableStages.delete(usPodInfo.Stage)
				p.cond.Broadcast()
			}
		} else {
			// Pod update didn't make it schedulable, keep it in the unschedulable queue.
			p.unschedulableStages.addOrUpdate(pInfo)
		}

		return nil
	}
	// If pod is not in any of the queues, we put it in the active queue.
	pInfo := p.newQueuedStageInfo(newStage)
	if err := p.activeQ.Add(pInfo); err != nil {
		return err
	}
	p.StageNominator.AddNominatedStage(pInfo.StageInfo, nil)
	p.cond.Broadcast()
	return nil
}

func (p *PriorityQueue) Delete(stage *meta.Stage) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.StageNominator.DeleteNominatedStageIfExists(stage)
	if err := p.activeQ.Delete(newQueuedStageInfoForLookup(stage)); err != nil {
		// The item was probably not found in the activeQ.
		p.podBackoffQ.Delete(newQueuedStageInfoForLookup(stage))
		p.unschedulableStages.delete(stage)
	}
	return nil
}

func (p *PriorityQueue) MoveAllToActiveOrBackoffQueue(event framework.ClusterEvent, preCheck PreEnqueueCheck) {
	p.lock.Lock()
	defer p.lock.Unlock()
	unschedulablePods := make([]*framework.QueuedStageInfo, 0, len(p.unschedulableStages.stageInfoMap))
	for _, pInfo := range p.unschedulableStages.stageInfoMap {
		if preCheck == nil || preCheck(pInfo.Stage) {
			unschedulablePods = append(unschedulablePods, pInfo)
		}
	}
	p.movePodsToActiveOrBackoffQueue(unschedulablePods, event)
}

// NOTE: this function assumes lock has been acquired in caller
func (p *PriorityQueue) movePodsToActiveOrBackoffQueue(podInfoList []*framework.QueuedStageInfo, event framework.ClusterEvent) {
	activated := false
	for _, pInfo := range podInfoList {
		// If the event doesn't help making the Pod schedulable, continue.
		// Note: we don't run the check if pInfo.UnschedulablePlugins is nil, which denotes
		// either there is some abnormal error, or scheduling the pod failed by plugins other than PreFilter, Filter and Permit.
		// In that case, it's desired to move it anyways.
		if len(pInfo.UnschedulablePlugins) != 0 && !p.podMatchesEvent(pInfo, event) {
			continue
		}
		pod := pInfo.Stage
		if p.isStageBackingoff(pInfo) {
			if err := p.podBackoffQ.Add(pInfo); err != nil {
				flog.Error(err)
				flog.Errorf("Error adding pod to the backoff queue, %v", pod)
			} else {
				p.unschedulableStages.delete(pod)
			}
		} else {
			if err := p.activeQ.Add(pInfo); err != nil {
				flog.Error(err)
				flog.Errorf("Error adding pod to the scheduling queue, %v", pod)
			} else {
				activated = true
				p.unschedulableStages.delete(pod)
			}
		}
	}
	p.moveRequestCycle = p.schedulingCycle
	if activated {
		p.cond.Broadcast()
	}
}

// Checks if the Pod may become schedulable upon the event.
// This is achieved by looking up the global clusterEventMap registry.
func (p *PriorityQueue) podMatchesEvent(podInfo *framework.QueuedStageInfo, clusterEvent framework.ClusterEvent) bool {
	if clusterEvent.IsWildCard() {
		return true
	}

	for evt, nameSet := range p.clusterEventMap {
		// Firstly verify if the two ClusterEvents match:
		// - either the registered event from plugin side is a WildCardEvent,
		// - or the two events have identical Resource fields and *compatible* ActionType.
		//   Note the ActionTypes don't need to be *identical*. We check if the ANDed value
		//   is zero or not. In this way, it's easy to tell Update&Delete is not compatible,
		//   but Update&All is.
		evtMatch := evt.IsWildCard() ||
			(evt.Resource == clusterEvent.Resource && evt.ActionType&clusterEvent.ActionType != 0)

		// Secondly verify the plugin name matches.
		// Note that if it doesn't match, we shouldn't continue to search.
		if evtMatch && intersect(nameSet, podInfo.UnschedulablePlugins) {
			return true
		}
	}

	return false
}

// getUnschedulablePodsWithMatchingAffinityTerm returns unschedulable pods which have
// any affinity term that matches "pod".
// NOTE: this function assumes lock has been acquired in caller.
func (p *PriorityQueue) getUnschedulablePodsWithMatchingAffinityTerm(stage *meta.Stage) []*framework.QueuedStageInfo {
	var podsToMove []*framework.QueuedStageInfo
	for _, pInfo := range p.unschedulableStages.stageInfoMap {
		///for _, term := range pInfo.RequiredAffinityTerms {
		//	if term.Matches(stage, nsLabels) {
		podsToMove = append(podsToMove, pInfo) // todo
		//		break
		//	}
		//}
	}
	return podsToMove
}

func (p *PriorityQueue) AssignedPodAdded(stage *meta.Stage) {
	p.lock.Lock()
	p.movePodsToActiveOrBackoffQueue(p.getUnschedulablePodsWithMatchingAffinityTerm(stage), AssignedStageAdd)
	p.lock.Unlock()
}

func (p *PriorityQueue) AssignedPodUpdated(stage *meta.Stage) {
	p.lock.Lock()
	p.movePodsToActiveOrBackoffQueue(p.getUnschedulablePodsWithMatchingAffinityTerm(stage), AssignedStageUpdate)
	p.lock.Unlock()
}

func (p *PriorityQueue) PendingPods() []*meta.Stage {
	p.lock.RLock()
	defer p.lock.RUnlock()

	var result []*meta.Stage
	for _, pInfo := range p.activeQ.List() {
		result = append(result, pInfo.(*framework.QueuedStageInfo).Stage)
	}
	for _, pInfo := range p.podBackoffQ.List() {
		result = append(result, pInfo.(*framework.QueuedStageInfo).Stage)
	}
	for _, pInfo := range p.unschedulableStages.stageInfoMap {
		result = append(result, pInfo.Stage)
	}
	return result
}

func (p *PriorityQueue) Close() {
	p.lock.Lock()
	defer p.lock.Unlock()

	close(p.stop)
	p.closed = true
	p.cond.Broadcast()
}

// flushBackoffQCompleted Moves all pods from backoffQ which have completed backoff in to activeQ
func (p *PriorityQueue) flushBackoffQCompleted() {
	p.lock.Lock()
	defer p.lock.Unlock()
	activated := false
	for {
		rawPodInfo := p.podBackoffQ.Peek()
		if rawPodInfo == nil {
			break
		}
		pod := rawPodInfo.(*framework.QueuedStageInfo).Stage
		boTime := p.getBackoffTime(rawPodInfo.(*framework.QueuedStageInfo))
		if boTime.After(p.clock.Now()) {
			break
		}
		_, err := p.podBackoffQ.Pop()
		if err != nil {
			flog.Error(err)
			flog.Errorf("Unable to pop pod from backoff queue despite backoff completion, %v", pod)
			break
		}
		p.activeQ.Add(rawPodInfo)
		activated = true
	}

	if activated {
		p.cond.Broadcast()
	}
}

// flushUnschedulablePodsLeftover moves pods which stay in unschedulablePods
// longer than podMaxInUnschedulablePodsDuration to backoffQ or activeQ.
func (p *PriorityQueue) flushUnschedulablePodsLeftover() {
	p.lock.Lock()
	defer p.lock.Unlock()

	var podsToMove []*framework.QueuedStageInfo
	currentTime := p.clock.Now()
	for _, pInfo := range p.unschedulableStages.stageInfoMap {
		lastScheduleTime := pInfo.Timestamp
		if currentTime.Sub(lastScheduleTime) > p.podMaxInUnschedulablePodsDuration {
			podsToMove = append(podsToMove, pInfo)
		}
	}

	if len(podsToMove) > 0 {
		p.movePodsToActiveOrBackoffQueue(podsToMove, UnschedulableTimeout)
	}
}

// NewStageNominator creates a nominator as a backing of framework.PodNominator.
// A podLister is passed in so as to check if the pod exists
// before adding its nominatedNode info.
func NewStageNominator(podLister interface{}) framework.StageNominator {
	return &nominator{
		podLister:          podLister,
		nominatedPods:      make(map[string][]*framework.StageInfo),
		nominatedPodToNode: make(map[string]string),
	}
}

type nominator struct {
	// podLister is used to verify if the given pod is alive.
	podLister interface{}
	// nominatedPods is a map keyed by a node name and the value is a list of
	// pods which are nominated to run on the node. These are pods which can be in
	// the activeQ or unschedulablePods.
	nominatedPods map[string][]*framework.StageInfo
	// nominatedPodToNode is map keyed by a Pod UID to the node name where it is
	// nominated.
	nominatedPodToNode map[string]string

	sync.RWMutex
}

func (npm *nominator) AddNominatedStage(stage *framework.StageInfo, nominatingInfo *framework.NominatingInfo) {
	npm.Lock()
	npm.add(stage, nominatingInfo)
	npm.Unlock()
}

func (npm *nominator) delete(p *meta.Stage) {
	nnn, ok := npm.nominatedPodToNode[p.UID]
	if !ok {
		return
	}
	for i, np := range npm.nominatedPods[nnn] {
		if np.Stage.UID == p.UID {
			npm.nominatedPods[nnn] = append(npm.nominatedPods[nnn][:i], npm.nominatedPods[nnn][i+1:]...)
			if len(npm.nominatedPods[nnn]) == 0 {
				delete(npm.nominatedPods, nnn)
			}
			break
		}
	}
	delete(npm.nominatedPodToNode, p.UID)
}

func (npm *nominator) add(pi *framework.StageInfo, nominatingInfo *framework.NominatingInfo) {
	// Always delete the pod if it already exists, to ensure we never store more than
	// one instance of the pod.
	npm.delete(pi.Stage)

	var nodeName string
	if nominatingInfo.Mode() == framework.ModeOverride {
		nodeName = nominatingInfo.NominatedNodeName
	} else if nominatingInfo.Mode() == framework.ModeNoop {
		//if pi.Stage.Status.NominatedNodeName == "" {
		//	return
		//}
		//nodeName = pi.Stage.Status.NominatedNodeName
	}

	if npm.podLister != nil {
		// If the pod was removed or if it was already scheduled, don't nominate it.
		//updatedPod, err := npm.podLister.Pods(pi.Pod.Namespace).Get(pi.Pod.Name)
		//if err != nil {
		//	klog.V(4).InfoS("Pod doesn't exist in podLister, aborted adding it to the nominator", "pod", klog.KObj(pi.Pod))
		//	return
		//}
		//if updatedPod.NodeName != "" {
		//	klog.V(4).InfoS("Pod is already scheduled to a node, aborted adding it to the nominator", "pod", klog.KObj(pi.Pod), "node", updatedPod.Spec.NodeName)
		//	return
		//}
	}

	npm.nominatedPodToNode[pi.Stage.UID] = nodeName
	for _, npi := range npm.nominatedPods[nodeName] {
		if npi.Stage.UID == pi.Stage.UID {
			flog.Infof("Pod already exists in the nominator, %v", npi.Stage)
			return
		}
	}
	npm.nominatedPods[nodeName] = append(npm.nominatedPods[nodeName], pi)
}

func (npm *nominator) DeleteNominatedStageIfExists(stage *meta.Stage) {
	npm.Lock()
	npm.delete(stage)
	npm.Unlock()
}

func NominatedStageName(pod *meta.Stage) string {
	return pod.Name // fixme pod.Status.NominatedNodeName
}

func (npm *nominator) UpdateNominatedStage(oldStage *meta.Stage, newStageInfo *framework.StageInfo) {
	npm.Lock()
	defer npm.Unlock()
	// In some cases, an Update event with no "NominatedNode" present is received right
	// after a node("NominatedNode") is reserved for this pod in memory.
	// In this case, we need to keep reserving the NominatedNode when updating the pod pointer.
	var nominatingInfo *framework.NominatingInfo
	// We won't fall into below `if` block if the Update event represents:
	// (1) NominatedNode info is added
	// (2) NominatedNode info is updated
	// (3) NominatedNode info is removed
	if NominatedStageName(oldStage) == "" && NominatedStageName(newStageInfo.Stage) == "" {
		if nnn, ok := npm.nominatedPodToNode[oldStage.UID]; ok {
			// This is the only case we should continue reserving the NominatedNode
			nominatingInfo = &framework.NominatingInfo{
				NominatingMode:    framework.ModeOverride,
				NominatedNodeName: nnn,
			}
		}
	}
	// We update irrespective of the nominatedNodeName changed or not, to ensure
	// that pod pointer is updated.
	npm.delete(oldStage)
	npm.add(newStageInfo, nominatingInfo)
}

func (npm *nominator) NominatedStagesForNode(nodeName string) []*framework.StageInfo {
	npm.RLock()
	defer npm.RUnlock()
	// Make a copy of the nominated Pods so the caller can mutate safely.
	pods := make([]*framework.StageInfo, len(npm.nominatedPods[nodeName]))
	for i := 0; i < len(pods); i++ {
		pods[i] = npm.nominatedPods[nodeName][i] // todo DeepCopy()
	}
	return pods
}

func (p *PriorityQueue) Run() {
	go parallelizer.JitterUntil(p.flushBackoffQCompleted, 1.0*time.Second, 0.0, true, p.stop)
	go parallelizer.JitterUntil(p.flushUnschedulablePodsLeftover, 30*time.Second, 0.0, true, p.stop)
}

// NewPriorityQueue creates a PriorityQueue object.
func NewPriorityQueue(
	lessFn framework.LessFunc,
	informerFactory interface{},
	opts ...Option,
) *PriorityQueue {
	options := defaultPriorityQueueOptions
	for _, opt := range opts {
		opt(&options)
	}

	comp := func(podInfo1, podInfo2 interface{}) bool {
		pInfo1 := podInfo1.(*framework.QueuedStageInfo)
		pInfo2 := podInfo2.(*framework.QueuedStageInfo)
		return lessFn(pInfo1, pInfo2)
	}

	if options.podNominator == nil {
		options.podNominator = NewStageNominator(nil)
	}

	pq := &PriorityQueue{
		StageNominator:                    options.podNominator,
		clock:                             options.clock,
		stop:                              make(chan struct{}),
		podInitialBackoffDuration:         options.podInitialBackoffDuration,
		podMaxBackoffDuration:             options.podMaxBackoffDuration,
		podMaxInUnschedulablePodsDuration: options.podMaxInUnschedulablePodsDuration,
		activeQ:                           heap.NewWithRecorder(podInfoKeyFunc, comp),
		unschedulableStages:               newUnschedulableStages(),
		moveRequestCycle:                  -1,
		clusterEventMap:                   options.clusterEventMap,
	}
	pq.cond.L = &pq.lock
	pq.podBackoffQ = heap.NewWithRecorder(podInfoKeyFunc, pq.podsCompareBackoffCompleted)
	// pq.nsLister = informerFactory.Core().V1().Namespaces().Lister()

	return pq
}

func (p *PriorityQueue) podsCompareBackoffCompleted(podInfo1, podInfo2 interface{}) bool {
	pInfo1 := podInfo1.(*framework.QueuedStageInfo)
	pInfo2 := podInfo2.(*framework.QueuedStageInfo)
	bo1 := p.getBackoffTime(pInfo1)
	bo2 := p.getBackoffTime(pInfo2)
	return bo1.Before(bo2)
}

func (p *PriorityQueue) getBackoffTime(podInfo *framework.QueuedStageInfo) time.Time {
	duration := p.calculateBackoffDuration(podInfo)
	backoffTime := podInfo.Timestamp.Add(duration)
	return backoffTime
}

func (p *PriorityQueue) calculateBackoffDuration(podInfo *framework.QueuedStageInfo) time.Duration {
	duration := p.podInitialBackoffDuration
	for i := 1; i < podInfo.Attempts; i++ {
		// Use subtraction instead of addition or multiplication to avoid overflow.
		if duration > p.podMaxBackoffDuration-duration {
			return p.podMaxBackoffDuration
		}
		duration += duration
	}
	return duration
}

// UnschedulableStages holds pods that cannot be scheduled. This data structure
// is used to implement unschedulablePods.
type UnschedulableStages struct {
	// podInfoMap is a map key by a pod's full-name and the value is a pointer to the QueuedPodInfo.
	stageInfoMap map[string]*framework.QueuedStageInfo
	keyFunc      func(stage *meta.Stage) string
}

// Add adds a pod to the unschedulable podInfoMap.
func (u *UnschedulableStages) addOrUpdate(pInfo *framework.QueuedStageInfo) {
	podID := u.keyFunc(pInfo.Stage)
	u.stageInfoMap[podID] = pInfo
}

// Delete deletes a pod from the unschedulable podInfoMap.
func (u *UnschedulableStages) delete(stage *meta.Stage) {
	podID := u.keyFunc(stage)
	delete(u.stageInfoMap, podID)
}

// Get returns the QueuedPodInfo if a pod with the same key as the key of the given "pod"
// is found in the map. It returns nil otherwise.
func (u *UnschedulableStages) get(stage *meta.Stage) *framework.QueuedStageInfo {
	podKey := u.keyFunc(stage)
	if pInfo, exists := u.stageInfoMap[podKey]; exists {
		return pInfo
	}
	return nil
}

// Clear removes all the entries from the unschedulable podInfoMap.
func (u *UnschedulableStages) clear() {
	u.stageInfoMap = make(map[string]*framework.QueuedStageInfo)
}

func podInfoKeyFunc(obj interface{}) (string, error) {
	return MetaNamespaceKeyFunc(obj.(*framework.QueuedStageInfo).Stage)
}

type ExplicitKey string

func MetaNamespaceKeyFunc(obj interface{}) (string, error) {
	if key, ok := obj.(ExplicitKey); ok {
		return string(key), nil
	}
	m, err := meta.Accessor(obj)
	if err != nil {
		return "", fmt.Errorf("object has no meta: %v", err)
	}
	return m.GetName(), nil
}

func newUnschedulableStages() *UnschedulableStages {
	return &UnschedulableStages{
		stageInfoMap: make(map[string]*framework.QueuedStageInfo),
		keyFunc:      GetStageFullName,
	}
}

func GetStageFullName(stage *meta.Stage) string {
	return stage.UID
}

func intersect(x, y map[string]struct{}) bool {
	if len(x) > len(y) {
		x, y = y, x
	}
	for v := range x {
		if _, ok := y[v]; ok {
			return true
		}
	}
	return false
}
