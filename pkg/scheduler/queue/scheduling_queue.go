package queue

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	v1 "github.com/tsundata/flowline/pkg/informer/listers/core/v1"
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
	// AssignedStageAdd is the event when a stage is added that causes stages with matching affinity terms
	// to be more schedulable.
	AssignedStageAdd = framework.ClusterEvent{Resource: framework.Stage, ActionType: framework.Add, Label: "AssignedStageAdd"}
	// WorkerAdd is the event when a new worker is added to the cluster.
	WorkerAdd = framework.ClusterEvent{Resource: framework.Worker, ActionType: framework.Add, Label: "WorkerAdd"}
	// AssignedStageUpdate is the event when a stage is updated that causes stages with matching affinity
	// terms to be more schedulable.
	AssignedStageUpdate = framework.ClusterEvent{Resource: framework.Stage, ActionType: framework.Update, Label: "AssignedStageUpdate"}
	// AssignedStageDelete is the event when a stage is deleted that causes stages with matching affinity
	// terms to be more schedulable.
	AssignedStageDelete = framework.ClusterEvent{Resource: framework.Stage, ActionType: framework.Delete, Label: "AssignedStageDelete"}
	// UnschedulableTimeout is the event when a stage stays in unschedulable for longer than timeout.
	UnschedulableTimeout = framework.ClusterEvent{Resource: framework.WildCard, ActionType: framework.All, Label: "UnschedulableTimeout"}
	// WorkerStateChange is the event when node label is changed.
	WorkerStateChange = framework.ClusterEvent{Resource: framework.Worker, ActionType: framework.Update, Label: "WorkerStateChange"}
)

// PreEnqueueCheck is a function type. It's used to build functions that
// run against a Stage and the caller can choose to enqueue or skip the Stage
// by the checking result.
type PreEnqueueCheck func(stage *meta.Stage) bool

type SchedulingQueue interface {
	framework.StageNominator
	Add(stage *meta.Stage) error
	// Activate moves the given stages to activeQ iff they're in unschedulableStages or backoffQ.
	// The passed-in stages are originally compiled from plugins that want to activate Stages,
	// by injecting the stages through a reserved CycleState struct (StagesToActivate).
	Activate(stages map[string]*meta.Stage)
	// AddUnschedulableIfNotPresent adds an unschedulable stage back to scheduling queue.
	// The stageSchedulingCycle represents the current scheduling cycle number which can be
	// returned by calling SchedulingCycle().
	AddUnschedulableIfNotPresent(stage *framework.QueuedStageInfo, stageSchedulingCycle int64) error
	// SchedulingCycle returns the current number of scheduling cycle which is
	// cached by scheduling queue. Normally, incrementing this number whenever
	// a stage is popped (e.g. called Pop()) is enough.
	SchedulingCycle() int64
	// Pop removes the head of the queue and returns it. It blocks if the
	// queue is empty and waits until a new item is added to the queue.
	Pop() (*framework.QueuedStageInfo, error)
	Update(oldStage, newStage *meta.Stage) error
	Delete(stage *meta.Stage) error
	MoveAllToActiveOrBackoffQueue(event framework.ClusterEvent, preCheck PreEnqueueCheck)
	AssignedStageAdded(stage *meta.Stage)
	AssignedStageUpdated(stage *meta.Stage)
	PendingStages() []*meta.Stage
	// Close closes the SchedulingQueue so that the goroutine which is
	// waiting to pop items can exit gracefully.
	Close()
	// Run starts the goroutines managing the queue.
	Run()
}

type priorityQueueOptions struct {
	clock                                 clock.Clock
	stageInitialBackoffDuration           time.Duration
	stageMaxBackoffDuration               time.Duration
	stageMaxInUnschedulableStagesDuration time.Duration
	stageNominator                        framework.StageNominator
	clusterEventMap                       map[framework.ClusterEvent]map[string]struct{}
}

const (
	// DefaultStageMaxInUnschedulableStagesDuration is the default value for the maximum
	// time a stage can stay in unschedulableStages. If a stage stays in unschedulableStages
	// for longer than this value, the stage will be moved from unschedulableStages to
	// backoffQ or activeQ. If this value is empty, the default value (5min)
	// will be used.
	DefaultStageMaxInUnschedulableStagesDuration time.Duration = 5 * time.Minute

	queueClosed = "scheduling queue is closed"
)

const (
	// DefaultStageInitialBackoffDuration is the default value for the initial backoff duration
	// for unschedulable stages. To change the default stageInitialBackoffDurationSeconds used by the
	// scheduler, update the ComponentConfig value in defaults.go
	DefaultStageInitialBackoffDuration time.Duration = 1 * time.Second
	// DefaultStageMaxBackoffDuration is the default value for the max backoff duration
	// for unschedulable stages. To change the default stageMaxBackoffDurationSeconds used by the
	// scheduler, update the ComponentConfig value in defaults.go
	DefaultStageMaxBackoffDuration time.Duration = 10 * time.Second
)

var defaultPriorityQueueOptions = priorityQueueOptions{
	clock:                                 clock.RealClock{},
	stageInitialBackoffDuration:           DefaultStageInitialBackoffDuration,
	stageMaxBackoffDuration:               DefaultStageMaxBackoffDuration,
	stageMaxInUnschedulableStagesDuration: DefaultStageMaxInUnschedulableStagesDuration,
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

func MakeNextStageFunc(queue SchedulingQueue) func() *framework.QueuedStageInfo {
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

// newQueuedStageInfoForLookup builds a QueuedStageInfo object for a lookup in the queue.
func newQueuedStageInfoForLookup(stage *meta.Stage, plugins ...string) *framework.QueuedStageInfo {
	sets := make(map[string]struct{})
	for _, plugin := range plugins {
		sets[plugin] = struct{}{}
	}
	// Since this is only used for a lookup in the queue, we only need to set the Stage,
	// and so we avoid creating a full StageInfo, which is expensive to instantiate frequently.
	return &framework.QueuedStageInfo{
		StageInfo:            &framework.StageInfo{Stage: stage},
		UnschedulablePlugins: sets,
	}
}

// PriorityQueue implements a scheduling queue.
// The head of PriorityQueue is the highest priority pending stage. This structure
// has two sub queues and a additional data structure, namely: activeQ,
// backoffQ and unschedulableStages.
// - activeQ holds stages that are being considered for scheduling.
// - backoffQ holds stages that moved from unschedulableStages and will move to
//   activeQ when their backoff periods complete.
// - unschedulableStages holds stages that were already attempted for scheduling and
//   are currently determined to be unschedulable.
type PriorityQueue struct {
	// StageNominator abstracts the operations to maintain nominated Stages.
	framework.StageNominator

	stop  chan struct{}
	clock clock.Clock

	// stage initial backoff duration.
	stageInitialBackoffDuration time.Duration
	// stage maximum backoff duration.
	stageMaxBackoffDuration time.Duration
	// the maximum time a stage can stay in the unschedulableStages.
	stageMaxInUnschedulableStagesDuration time.Duration

	lock sync.RWMutex
	cond sync.Cond

	// activeQ is heap structure that scheduler actively looks at to find stages to
	// schedule. Head of heap is the highest priority stage.
	activeQ *heap.Heap
	// stageBackoffQ is a heap ordered by backoff expiry. Stages which have completed backoff
	// are popped from this heap before the scheduler looks at activeQ
	stageBackoffQ *heap.Heap
	// unschedulableStages holds stages that have been tried and determined unschedulable.
	unschedulableStages *UnschedulableStages
	// schedulingCycle represents sequence number of scheduling cycle and is incremented
	// when a stage is popped.
	schedulingCycle int64
	// moveRequestCycle caches the sequence number of scheduling cycle when we
	// received a move request. Unschedulable stages in and before this scheduling
	// cycle will be put back to activeQueue if we were trying to schedule them
	// when we received move request.
	moveRequestCycle int64

	clusterEventMap map[framework.ClusterEvent]map[string]struct{}

	// closed indicates that the queue is closed.
	// It is mainly used to let Pop() exit its control loop while waiting for an item.
	closed bool

	nsLister interface{}
}

// newQueuedStageInfo builds a QueuedStageInfo object.
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
	// Verify if the stage is present in activeQ.
	if _, exists, _ := p.activeQ.Get(newQueuedStageInfoForLookup(stage)); exists {
		// No need to activate if it's already present in activeQ.
		return false
	}
	var pInfo *framework.QueuedStageInfo
	// Verify if the stage is present in unschedulableStages or backoffQ.
	if pInfo = p.unschedulableStages.get(stage); pInfo == nil {
		// If the stage doesn't belong to unschedulableStages or backoffQ, don't activate it.
		if obj, exists, _ := p.stageBackoffQ.Get(newQueuedStageInfoForLookup(stage)); !exists {
			flog.Errorf("To-activate stage does not exist in unschedulableStages or backoffQ, %v", stage)
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
		flog.Errorf("Error adding stage to the scheduling queue, %v", stage)
		return false
	}
	p.unschedulableStages.delete(stage)
	p.stageBackoffQ.Delete(pInfo)
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
	// Delete stage from backoffQ if it is backing off
	if err := p.stageBackoffQ.Delete(pInfo); err == nil {
		flog.Errorf("Error: stage is already in the stageBackoff queue, %v", stage)
	}
	p.StageNominator.AddNominatedStage(pInfo.StageInfo, nil)
	p.cond.Broadcast()

	return nil
}

func (p *PriorityQueue) Activate(stages map[string]*meta.Stage) {
	p.lock.Lock()
	defer p.lock.Unlock()

	activated := false
	for _, stage := range stages {
		if p.activate(stage) {
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
	if _, exists, _ := p.stageBackoffQ.Get(pInfo); exists {
		return fmt.Errorf("stage %v is already present in the backoff queue", stage)
	}

	// Refresh the timestamp since the stage is re-added.
	pInfo.Timestamp = p.clock.Now()

	// If a move request has been received, move it to the BackoffQ, otherwise move
	// it to unschedulableStages.
	if p.moveRequestCycle >= stageSchedulingCycle {
		if err := p.stageBackoffQ.Add(pInfo); err != nil {
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

func updateStage(oldStageInfo interface{}, newStage *meta.Stage) *framework.QueuedStageInfo {
	pInfo := oldStageInfo.(*framework.QueuedStageInfo)
	pInfo.Update(newStage)
	return pInfo
}

// isStageUpdated checks if the stage is updated in a way that it may have become
// schedulable. It drops status of the stage and compares it with old version.
func isStageUpdated(oldStage, newStage *meta.Stage) bool {
	strip := func(stage *meta.Stage) *meta.Stage {
		p := stage // todo deep copy
		p.ResourceVersion = ""
		p.Generation = 0
		//p.Status = v1.StageStatus{}
		//p.ManagedFields = nil
		p.Finalizers = nil
		return p
	}
	return !reflect.DeepEqual(strip(oldStage), strip(newStage))
}

// isStageBackingoff returns true if a stage is still waiting for its backoff timer.
// If this returns true, the stage should not be re-tried.
func (p *PriorityQueue) isStageBackingoff(stageInfo *framework.QueuedStageInfo) bool {
	boTime := p.getBackoffTime(stageInfo)
	return boTime.After(p.clock.Now())
}

func (p *PriorityQueue) Update(oldStage, newStage *meta.Stage) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if oldStage != nil {
		oldStageInfo := newQueuedStageInfoForLookup(oldStage)
		// If the stage is already in the active queue, just update it there.
		if oldStageInfo, exists, _ := p.activeQ.Get(oldStageInfo); exists {
			pInfo := updateStage(oldStageInfo, newStage)
			p.StageNominator.UpdateNominatedStage(oldStage, pInfo.StageInfo)
			return p.activeQ.Update(pInfo)
		}

		// If the stage is in the backoff queue, update it there.
		if oldStageInfo, exists, _ := p.stageBackoffQ.Get(oldStageInfo); exists {
			pInfo := updateStage(oldStageInfo, newStage)
			p.StageNominator.UpdateNominatedStage(oldStage, pInfo.StageInfo)
			return p.stageBackoffQ.Update(pInfo)
		}
	}

	// If the stage is in the unschedulable queue, updating it may make it schedulable.
	if usStageInfo := p.unschedulableStages.get(newStage); usStageInfo != nil {
		pInfo := updateStage(usStageInfo, newStage)
		p.StageNominator.UpdateNominatedStage(oldStage, pInfo.StageInfo)
		if isStageUpdated(oldStage, newStage) {
			if p.isStageBackingoff(usStageInfo) {
				if err := p.stageBackoffQ.Add(pInfo); err != nil {
					return err
				}
				p.unschedulableStages.delete(usStageInfo.Stage)
			} else {
				if err := p.activeQ.Add(pInfo); err != nil {
					return err
				}
				p.unschedulableStages.delete(usStageInfo.Stage)
				p.cond.Broadcast()
			}
		} else {
			// Stage update didn't make it schedulable, keep it in the unschedulable queue.
			p.unschedulableStages.addOrUpdate(pInfo)
		}

		return nil
	}
	// If stage is not in any of the queues, we put it in the active queue.
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
		p.stageBackoffQ.Delete(newQueuedStageInfoForLookup(stage))
		p.unschedulableStages.delete(stage)
	}
	return nil
}

func (p *PriorityQueue) MoveAllToActiveOrBackoffQueue(event framework.ClusterEvent, preCheck PreEnqueueCheck) {
	p.lock.Lock()
	defer p.lock.Unlock()
	unschedulableStages := make([]*framework.QueuedStageInfo, 0, len(p.unschedulableStages.stageInfoMap))
	for _, pInfo := range p.unschedulableStages.stageInfoMap {
		if preCheck == nil || preCheck(pInfo.Stage) {
			unschedulableStages = append(unschedulableStages, pInfo)
		}
	}
	p.moveStagesToActiveOrBackoffQueue(unschedulableStages, event)
}

// NOTE: this function assumes lock has been acquired in caller
func (p *PriorityQueue) moveStagesToActiveOrBackoffQueue(stageInfoList []*framework.QueuedStageInfo, event framework.ClusterEvent) {
	activated := false
	for _, pInfo := range stageInfoList {
		// If the event doesn't help making the Stage schedulable, continue.
		// Note: we don't run the check if pInfo.UnschedulablePlugins is nil, which denotes
		// either there is some abnormal error, or scheduling the stage failed by plugins other than PreFilter, Filter and Permit.
		// In that case, it's desired to move it anyways.
		if len(pInfo.UnschedulablePlugins) != 0 && !p.stageMatchesEvent(pInfo, event) {
			continue
		}
		stage := pInfo.Stage
		if p.isStageBackingoff(pInfo) {
			if err := p.stageBackoffQ.Add(pInfo); err != nil {
				flog.Error(err)
				flog.Errorf("Error adding stage to the backoff queue, %v", stage)
			} else {
				p.unschedulableStages.delete(stage)
			}
		} else {
			if err := p.activeQ.Add(pInfo); err != nil {
				flog.Error(err)
				flog.Errorf("Error adding stage to the scheduling queue, %v", stage)
			} else {
				activated = true
				p.unschedulableStages.delete(stage)
			}
		}
	}
	p.moveRequestCycle = p.schedulingCycle
	if activated {
		p.cond.Broadcast()
	}
}

// Checks if the Stage may become schedulable upon the event.
// This is achieved by looking up the global clusterEventMap registry.
func (p *PriorityQueue) stageMatchesEvent(stageInfo *framework.QueuedStageInfo, clusterEvent framework.ClusterEvent) bool {
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
		if evtMatch && intersect(nameSet, stageInfo.UnschedulablePlugins) {
			return true
		}
	}

	return false
}

// getUnschedulableStagesWithMatchingAffinityTerm returns unschedulable stages which have
// any affinity term that matches "stage".
// NOTE: this function assumes lock has been acquired in caller.
func (p *PriorityQueue) getUnschedulableStagesWithMatchingAffinityTerm(stage *meta.Stage) []*framework.QueuedStageInfo {
	var stagesToMove []*framework.QueuedStageInfo
	for _, pInfo := range p.unschedulableStages.stageInfoMap {
		///for _, term := range pInfo.RequiredAffinityTerms {
		//	if term.Matches(stage, nsLabels) {
		stagesToMove = append(stagesToMove, pInfo) // todo
		//		break
		//	}
		//}
	}
	return stagesToMove
}

func (p *PriorityQueue) AssignedStageAdded(stage *meta.Stage) {
	p.lock.Lock()
	p.moveStagesToActiveOrBackoffQueue(p.getUnschedulableStagesWithMatchingAffinityTerm(stage), AssignedStageAdd)
	p.lock.Unlock()
}

func (p *PriorityQueue) AssignedStageUpdated(stage *meta.Stage) {
	p.lock.Lock()
	p.moveStagesToActiveOrBackoffQueue(p.getUnschedulableStagesWithMatchingAffinityTerm(stage), AssignedStageUpdate)
	p.lock.Unlock()
}

func (p *PriorityQueue) PendingStages() []*meta.Stage {
	p.lock.RLock()
	defer p.lock.RUnlock()

	var result []*meta.Stage
	for _, pInfo := range p.activeQ.List() {
		result = append(result, pInfo.(*framework.QueuedStageInfo).Stage)
	}
	for _, pInfo := range p.stageBackoffQ.List() {
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

// flushBackoffQCompleted Moves all stages from backoffQ which have completed backoff in to activeQ
func (p *PriorityQueue) flushBackoffQCompleted() {
	p.lock.Lock()
	defer p.lock.Unlock()
	activated := false
	for {
		rawStageInfo := p.stageBackoffQ.Peek()
		if rawStageInfo == nil {
			break
		}
		stage := rawStageInfo.(*framework.QueuedStageInfo).Stage
		boTime := p.getBackoffTime(rawStageInfo.(*framework.QueuedStageInfo))
		if boTime.After(p.clock.Now()) {
			break
		}
		_, err := p.stageBackoffQ.Pop()
		if err != nil {
			flog.Error(err)
			flog.Errorf("Unable to pop stage from backoff queue despite backoff completion, %v", stage)
			break
		}
		p.activeQ.Add(rawStageInfo)
		activated = true
	}

	if activated {
		p.cond.Broadcast()
	}
}

// flushUnschedulableStagesLeftover moves stages which stay in unschedulableStages
// longer than stageMaxInUnschedulableStagesDuration to backoffQ or activeQ.
func (p *PriorityQueue) flushUnschedulableStagesLeftover() {
	p.lock.Lock()
	defer p.lock.Unlock()

	var stagesToMove []*framework.QueuedStageInfo
	currentTime := p.clock.Now()
	for _, pInfo := range p.unschedulableStages.stageInfoMap {
		lastScheduleTime := pInfo.Timestamp
		if currentTime.Sub(lastScheduleTime) > p.stageMaxInUnschedulableStagesDuration {
			stagesToMove = append(stagesToMove, pInfo)
		}
	}

	if len(stagesToMove) > 0 {
		p.moveStagesToActiveOrBackoffQueue(stagesToMove, UnschedulableTimeout)
	}
}

// NewStageNominator creates a nominator as a backing of framework.StageNominator.
// A stageLister is passed in so as to check if the stage exists
// before adding its nominatedWorker info.
func NewStageNominator(stageLister v1.StageLister) framework.StageNominator {
	return &nominator{
		stageLister:            stageLister,
		nominatedStages:        make(map[string][]*framework.StageInfo),
		nominatedStageToWorker: make(map[string]string),
	}
}

type nominator struct {
	// stageLister is used to verify if the given stage is alive.
	stageLister v1.StageLister
	// nominatedStages is a map keyed by a worker name and the value is a list of
	// stages which are nominated to run on the worker. These are stages which can be in
	// the activeQ or unschedulableStages.
	nominatedStages map[string][]*framework.StageInfo
	// nominatedStageToWorker is map keyed by a Stage UID to the worker name where it is
	// nominated.
	nominatedStageToWorker map[string]string

	sync.RWMutex
}

func (npm *nominator) AddNominatedStage(stage *framework.StageInfo, nominatingInfo *framework.NominatingInfo) {
	npm.Lock()
	npm.add(stage, nominatingInfo)
	npm.Unlock()
}

func (npm *nominator) delete(p *meta.Stage) {
	nnn, ok := npm.nominatedStageToWorker[p.UID]
	if !ok {
		return
	}
	for i, np := range npm.nominatedStages[nnn] {
		if np.Stage.UID == p.UID {
			npm.nominatedStages[nnn] = append(npm.nominatedStages[nnn][:i], npm.nominatedStages[nnn][i+1:]...)
			if len(npm.nominatedStages[nnn]) == 0 {
				delete(npm.nominatedStages, nnn)
			}
			break
		}
	}
	delete(npm.nominatedStageToWorker, p.UID)
}

func (npm *nominator) add(pi *framework.StageInfo, nominatingInfo *framework.NominatingInfo) {
	// Always delete the stage if it already exists, to ensure we never store more than
	// one instance of the stage.
	npm.delete(pi.Stage)

	var workerName string
	if nominatingInfo.Mode() == framework.ModeOverride {
		workerName = nominatingInfo.NominatedWorkerName
	} else if nominatingInfo.Mode() == framework.ModeNoop {
		//if pi.Stage.Status.NominatedWorkerName == "" {
		//	return
		//}
		//workerName = pi.Stage.Status.NominatedWorkerName
	}

	if npm.stageLister != nil {
		//If the stage was removed or if it was already scheduled, don't nominate it.
		updatedStage, err := npm.stageLister.Get(pi.Stage.Name)
		if err != nil {
			flog.Errorf("Stage doesn't exist in stageLister, aborted adding it to the nominator %T", pi.Stage)
			return
		}
		if updatedStage.WorkerUID != "" {
			flog.Infof("Stage is already scheduled to a worker, aborted adding it to the nominator, %T %s", pi.Stage, updatedStage.WorkerUID)
			return
		}
	}

	npm.nominatedStageToWorker[pi.Stage.UID] = workerName
	for _, npi := range npm.nominatedStages[workerName] {
		if npi.Stage.UID == pi.Stage.UID {
			flog.Infof("Stage already exists in the nominator, %v", npi.Stage)
			return
		}
	}
	npm.nominatedStages[workerName] = append(npm.nominatedStages[workerName], pi)
}

func (npm *nominator) DeleteNominatedStageIfExists(stage *meta.Stage) {
	npm.Lock()
	npm.delete(stage)
	npm.Unlock()
}

func NominatedStageName(stage *meta.Stage) string {
	return stage.Name // fixme stage.Status.NominatedWorkerName
}

func (npm *nominator) UpdateNominatedStage(oldStage *meta.Stage, newStageInfo *framework.StageInfo) {
	npm.Lock()
	defer npm.Unlock()
	// In some cases, an Update event with no "NominatedWorker" present is received right
	// after a worker("NominatedWorker") is reserved for this stage in memory.
	// In this case, we need to keep reserving the NominatedWorker when updating the stage pointer.
	var nominatingInfo *framework.NominatingInfo
	// We won't fall into below `if` block if the Update event represents:
	// (1) NominatedWorker info is added
	// (2) NominatedWorker info is updated
	// (3) NominatedWorker info is removed
	if NominatedStageName(oldStage) == "" && NominatedStageName(newStageInfo.Stage) == "" {
		if nnn, ok := npm.nominatedStageToWorker[oldStage.UID]; ok {
			// This is the only case we should continue reserving the NominatedWorker
			nominatingInfo = &framework.NominatingInfo{
				NominatingMode:      framework.ModeOverride,
				NominatedWorkerName: nnn,
			}
		}
	}
	// We update irrespective of the nominatedWorkerName changed or not, to ensure
	// that stage pointer is updated.
	npm.delete(oldStage)
	npm.add(newStageInfo, nominatingInfo)
}

func (npm *nominator) NominatedStagesForWorker(workerName string) []*framework.StageInfo {
	npm.RLock()
	defer npm.RUnlock()
	// Make a copy of the nominated Stages so the caller can mutate safely.
	stages := make([]*framework.StageInfo, len(npm.nominatedStages[workerName]))
	for i := 0; i < len(stages); i++ {
		stages[i] = npm.nominatedStages[workerName][i] // todo DeepCopy()
	}
	return stages
}

func (p *PriorityQueue) Run() {
	go parallelizer.JitterUntil(p.flushBackoffQCompleted, 1.0*time.Second, 0.0, true, p.stop)
	go parallelizer.JitterUntil(p.flushUnschedulableStagesLeftover, 30*time.Second, 0.0, true, p.stop)
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

	comp := func(stageInfo1, stageInfo2 interface{}) bool {
		pInfo1 := stageInfo1.(*framework.QueuedStageInfo)
		pInfo2 := stageInfo2.(*framework.QueuedStageInfo)
		return lessFn(pInfo1, pInfo2)
	}

	if options.stageNominator == nil {
		options.stageNominator = NewStageNominator(nil)
	}

	pq := &PriorityQueue{
		StageNominator:                        options.stageNominator,
		clock:                                 options.clock,
		stop:                                  make(chan struct{}),
		stageInitialBackoffDuration:           options.stageInitialBackoffDuration,
		stageMaxBackoffDuration:               options.stageMaxBackoffDuration,
		stageMaxInUnschedulableStagesDuration: options.stageMaxInUnschedulableStagesDuration,
		activeQ:                               heap.NewWithRecorder(stageInfoKeyFunc, comp),
		unschedulableStages:                   newUnschedulableStages(),
		moveRequestCycle:                      -1,
		clusterEventMap:                       options.clusterEventMap,
	}
	pq.cond.L = &pq.lock
	pq.stageBackoffQ = heap.NewWithRecorder(stageInfoKeyFunc, pq.stagesCompareBackoffCompleted)
	// pq.nsLister = informerFactory.Core().V1().Namespaces().Lister()

	return pq
}

func (p *PriorityQueue) stagesCompareBackoffCompleted(stageInfo1, stageInfo2 interface{}) bool {
	pInfo1 := stageInfo1.(*framework.QueuedStageInfo)
	pInfo2 := stageInfo2.(*framework.QueuedStageInfo)
	bo1 := p.getBackoffTime(pInfo1)
	bo2 := p.getBackoffTime(pInfo2)
	return bo1.Before(bo2)
}

func (p *PriorityQueue) getBackoffTime(stageInfo *framework.QueuedStageInfo) time.Time {
	duration := p.calculateBackoffDuration(stageInfo)
	backoffTime := stageInfo.Timestamp.Add(duration)
	return backoffTime
}

func (p *PriorityQueue) calculateBackoffDuration(stageInfo *framework.QueuedStageInfo) time.Duration {
	duration := p.stageInitialBackoffDuration
	for i := 1; i < stageInfo.Attempts; i++ {
		// Use subtraction instead of addition or multiplication to avoid overflow.
		if duration > p.stageMaxBackoffDuration-duration {
			return p.stageMaxBackoffDuration
		}
		duration += duration
	}
	return duration
}

// UnschedulableStages holds stages that cannot be scheduled. This data structure
// is used to implement unschedulableStages.
type UnschedulableStages struct {
	// stageInfoMap is a map key by a stage's full-name and the value is a pointer to the QueuedStageInfo.
	stageInfoMap map[string]*framework.QueuedStageInfo
	keyFunc      func(stage *meta.Stage) string
}

// Add adds a stage to the unschedulable stageInfoMap.
func (u *UnschedulableStages) addOrUpdate(pInfo *framework.QueuedStageInfo) {
	stageID := u.keyFunc(pInfo.Stage)
	u.stageInfoMap[stageID] = pInfo
}

// Delete deletes a stage from the unschedulable stageInfoMap.
func (u *UnschedulableStages) delete(stage *meta.Stage) {
	stageID := u.keyFunc(stage)
	delete(u.stageInfoMap, stageID)
}

// Get returns the QueuedStageInfo if a stage with the same key as the key of the given "stage"
// is found in the map. It returns nil otherwise.
func (u *UnschedulableStages) get(stage *meta.Stage) *framework.QueuedStageInfo {
	stageKey := u.keyFunc(stage)
	if pInfo, exists := u.stageInfoMap[stageKey]; exists {
		return pInfo
	}
	return nil
}

// Clear removes all the entries from the unschedulable stageInfoMap.
func (u *UnschedulableStages) clear() {
	u.stageInfoMap = make(map[string]*framework.QueuedStageInfo)
}

func stageInfoKeyFunc(obj interface{}) (string, error) {
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
