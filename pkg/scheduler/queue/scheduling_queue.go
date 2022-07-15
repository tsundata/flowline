package queue

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/scheduler/heap"
	"github.com/tsundata/flowline/pkg/util/clock"
	"github.com/tsundata/flowline/pkg/util/flog"
	"sync"
	"time"
)

// terms to be more schedulable.
var AssignedPodDelete = framework.ClusterEvent{Resource: framework.Stage, ActionType: framework.Delete, Label: "AssignedPodDelete"}

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
			flog.Infof("About to try and schedule pod %v", stageInfo.Stage)
			return stageInfo
		}
		flog.Errorf("%s Error while retrieving next pod from scheduling queue", err)
		return nil
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
	unschedulablePods *UnschedulablePods
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

func (p *PriorityQueue) Add(stage *meta.Stage) error {
	//TODO implement me
	panic("implement me")
}

func (p *PriorityQueue) Activate(stages map[string]*meta.Stage) {
	//TODO implement me
	panic("implement me")
}

func (p *PriorityQueue) AddUnschedulableIfNotPresent(stage *framework.QueuedStageInfo, stageSchedulingCycle int64) error {
	//TODO implement me
	panic("implement me")
}

func (p *PriorityQueue) SchedulingCycle() int64 {
	//TODO implement me
	panic("implement me")
}

func (p *PriorityQueue) Pop() (*framework.QueuedStageInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (p *PriorityQueue) Update(oldStage, newStage *meta.Stage) error {
	//TODO implement me
	panic("implement me")
}

func (p *PriorityQueue) Delete(stage *meta.Stage) error {
	//TODO implement me
	panic("implement me")
}

func (p *PriorityQueue) MoveAllToActiveOrBackoffQueue(event framework.ClusterEvent, preCheck PreEnqueueCheck) {
	//TODO implement me
	panic("implement me")
}

func (p *PriorityQueue) AssignedPodAdded(stage *meta.Stage) {
	//TODO implement me
	panic("implement me")
}

func (p *PriorityQueue) AssignedPodUpdated(stage *meta.Stage) {
	//TODO implement me
	panic("implement me")
}

func (p *PriorityQueue) PendingPods() []*meta.Stage {
	//TODO implement me
	panic("implement me")
}

func (p *PriorityQueue) Close() {
	//TODO implement me
	panic("implement me")
}

func (p *PriorityQueue) Run() {
	//TODO implement me
	panic("implement me")
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
		// todo options.podNominator = NewStageNominator(informerFactory.Core().V1().Pods().Lister())
	}

	pq := &PriorityQueue{
		StageNominator:                    options.podNominator,
		clock:                             options.clock,
		stop:                              make(chan struct{}),
		podInitialBackoffDuration:         options.podInitialBackoffDuration,
		podMaxBackoffDuration:             options.podMaxBackoffDuration,
		podMaxInUnschedulablePodsDuration: options.podMaxInUnschedulablePodsDuration,
		activeQ:                           heap.NewWithRecorder(podInfoKeyFunc, comp),
		unschedulablePods:                 newUnschedulablePods(),
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

// UnschedulablePods holds pods that cannot be scheduled. This data structure
// is used to implement unschedulablePods.
type UnschedulablePods struct {
	// podInfoMap is a map key by a pod's full-name and the value is a pointer to the QueuedPodInfo.
	stageInfoMap map[string]*framework.QueuedStageInfo
	keyFunc      func(stage *meta.Stage) string
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

func newUnschedulablePods() *UnschedulablePods {
	return &UnschedulablePods{
		stageInfoMap: make(map[string]*framework.QueuedStageInfo),
		keyFunc:      GetPodFullName,
	}
}

func GetPodFullName(stage *meta.Stage) string {
	// Use underscore as the delimiter because it is not allowed in pod name
	// (DNS subdomain format).
	return stage.Name
}
