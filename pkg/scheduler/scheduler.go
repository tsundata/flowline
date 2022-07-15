package scheduler

import (
	"context"
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/scheduler/cache"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/scheduler/framework/config"
	"github.com/tsundata/flowline/pkg/scheduler/framework/plugins"
	frameworkruntime "github.com/tsundata/flowline/pkg/scheduler/framework/runtime"
	"github.com/tsundata/flowline/pkg/scheduler/queue"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"sync"
	"sync/atomic"
	"time"
)

// ScheduleResult represents the result of scheduling a pod.
type ScheduleResult struct {
	// Name of the selected node.
	SuggestedHost string
	// The number of nodes the scheduler evaluated the pod against in the filtering
	// phase and beyond.
	EvaluatedWorkers int
	// The number of nodes out of the evaluated ones that fit the pod.
	FeasibleWorkers int
}

type Scheduler struct {
	Cache cache.Cache

	Extenders []framework.Extender

	NextStage func() *framework.QueuedStageInfo

	Error func(*framework.QueuedStageInfo, error)

	ScheduleStage func(ctx context.Context, fwk framework.Framework, state *framework.CycleState, stage *meta.Stage) (ScheduleResult, error)

	StopEverything <-chan struct{}

	SchedulingQueue queue.SchedulingQueue

	// Profiles are the scheduling profiles.
	Profiles map[string]framework.Framework

	client interface{}

	percentageOfWorkersToScore int32
	nextStartWorkerIndex       int
}

type schedulerOptions struct {
	componentConfigVersion              string
	config                              interface{} //*restclient.Config
	percentageOfNodesToScore            int32
	stageInitialBackoffSeconds          int64
	stageMaxBackoffSeconds              int64
	stageMaxInUnschedulablePodsDuration time.Duration
	// Contains out-of-tree plugins to be merged with the in-tree registry.
	frameworkOutOfTreeRegistry frameworkruntime.Registry
	profiles                   []config.Profile
	extenders                  []config.Extender
	frameworkCapturer          FrameworkCapturer
	parallelism                int32
	applyDefaultProfile        bool
}

// Option configures a Scheduler
type Option func(*schedulerOptions)

// FrameworkCapturer is used for registering a notify function in building framework.
type FrameworkCapturer func(config.Profile)

func (sched *Scheduler) scheduleStage(ctx context.Context, fwk framework.Framework, state *framework.CycleState, stage *meta.Stage) (result ScheduleResult, err error) {

	return ScheduleResult{}, nil
}

// Filters the nodes to find the ones that fit the pod based on the framework
// filter plugins and filter extenders.
func (sched *Scheduler) findNodesThatFitPod(ctx context.Context, fwk framework.Framework, state *framework.CycleState, stage *meta.Stage) ([]*meta.Worker, framework.Diagnosis, error) {
	diagnosis := framework.Diagnosis{
		WorkerToStatusMap:    make(framework.WorkerToStatusMap),
		UnschedulablePlugins: make(map[string]struct{}),
	}

	var allWorkers []*framework.WorkerInfo //fixme get all workers

	workers := allWorkers
	feasibleWorkers, err := sched.findWorkersThatPassFilters(ctx, fwk, state, stage, diagnosis, workers)
	processedWorkers := len(feasibleWorkers) + len(diagnosis.WorkerToStatusMap)
	sched.nextStartWorkerIndex = (sched.nextStartWorkerIndex + processedWorkers) % len(workers)
	if err != nil {
		return nil, diagnosis, err
	}

	feasibleWorkers, err = findNodesThatPassExtenders(sched.Extenders, stage, feasibleWorkers, diagnosis.WorkerToStatusMap)
	if err != nil {
		return nil, diagnosis, err
	}
	return feasibleWorkers, diagnosis, nil
}

// findNodesThatPassFilters finds the nodes that fit the filter plugins.
func (sched *Scheduler) findWorkersThatPassFilters(
	ctx context.Context,
	fwk framework.Framework,
	state *framework.CycleState,
	stage *meta.Stage,
	diagnosis framework.Diagnosis,
	workers []*framework.WorkerInfo) ([]*meta.Worker, error) {

	numAllWorkers := len(workers)

	feasibleWorkers := make([]*meta.Worker, numAllWorkers)

	if !fwk.HasFilterPlugins() {
		for i := range feasibleWorkers {
			feasibleWorkers[i] = workers[(sched.nextStartWorkerIndex+i)%numAllWorkers].Worker()
		}
		return feasibleWorkers, nil
	}

	errCh := parallelizer.NewErrorChannel()
	var statusesLock sync.Mutex
	var feasibleWorkersLen int32
	ctx, cancel := context.WithCancel(ctx)
	checkWorker := func(i int) {
		workerInfo := workers[(sched.nextStartWorkerIndex+i)%numAllWorkers]
		status := fwk.RunFilterPluginsWithNominatedPods(ctx, state, stage, workerInfo)
		if status.Code() == framework.Error {
			errCh.SendErrorWithCancel(status.AsError(), cancel)
			return
		}
		if status.IsSuccess() {
			length := atomic.AddInt32(&feasibleWorkersLen, 1)
			feasibleWorkers[length-1] = workerInfo.Worker()
		} else {
			statusesLock.Lock()
			diagnosis.WorkerToStatusMap[workerInfo.Worker().UID] = status
			diagnosis.UnschedulablePlugins[status.FailedPlugin()] = struct{}{}
			statusesLock.Unlock()
		}
	}

	parallelizer.ParallelizeUntil(ctx, 10, numAllWorkers, checkWorker)
	feasibleWorkers = feasibleWorkers[:feasibleWorkersLen]
	if err := errCh.ReceiveError(); err != nil {
		return feasibleWorkers, err
	}

	return feasibleWorkers, nil
}

func findNodesThatPassExtenders(extenders []framework.Extender, stage *meta.Stage, feasibleWorkers []*meta.Worker, statuses framework.WorkerToStatusMap) ([]*meta.Worker, error) {
	for _, extender := range extenders {
		if len(feasibleWorkers) == 0 {
			break
		}
		if !extender.IsInterested(stage) {
			continue
		}

		feasibleList, failedMap, failedAndUnresolvableMap, err := extender.Filter(stage, feasibleWorkers)
		if err != nil {
			if extender.IsIgnorable() {
				flog.Infof("Skipping extender as it returned error and has ignorable flag set %v %s", extender, err)
				continue
			}
			return nil, err
		}

		for failedWorkerName, failedMsg := range failedAndUnresolvableMap {
			var aggregatedReasons []string
			if _, found := statuses[failedWorkerName]; found {
				aggregatedReasons = statuses[failedWorkerName].Reasons()
			}
			aggregatedReasons = append(aggregatedReasons, failedMsg)
			statuses[failedWorkerName] = framework.NewStatus(framework.UnschedulableAndUnresolvable, aggregatedReasons...)
		}

		for failedWorkerName, failedMsg := range failedMap {
			if _, found := failedAndUnresolvableMap[failedWorkerName]; found {
				continue
			}
			if _, found := statuses[failedWorkerName]; !found {
				statuses[failedWorkerName] = framework.NewStatus(framework.Unschedulable, failedMsg)
			} else {
				statuses[failedWorkerName].AppendReason(failedMsg)
			}
		}

		feasibleWorkers = feasibleList
	}
	return feasibleWorkers, nil
}

// newScheduler creates a Scheduler object.
func newScheduler(
	cache cache.Cache,
	extenders []framework.Extender,
	nextStage func() *framework.QueuedStageInfo,
	Error func(*framework.QueuedStageInfo, error),
	stopEverything <-chan struct{},
	schedulingQueue queue.SchedulingQueue,
	profiles map[string]framework.Framework,
	client interface{},
	percentageOfNodesToScore int32) *Scheduler {
	sched := Scheduler{
		Cache:                      cache,
		Extenders:                  extenders,
		NextStage:                  nextStage,
		Error:                      Error,
		StopEverything:             stopEverything,
		SchedulingQueue:            schedulingQueue,
		Profiles:                   profiles,
		client:                     client,
		percentageOfWorkersToScore: percentageOfNodesToScore,
	}
	sched.ScheduleStage = sched.scheduleStage
	return &sched
}

// NeverStop may be passed to Until to make it never stop.
var NeverStop <-chan struct{} = make(chan struct{})

var defaultSchedulerOptions = schedulerOptions{
	percentageOfNodesToScore:            0,
	stageInitialBackoffSeconds:          int64((1 * time.Second).Seconds()),
	stageMaxBackoffSeconds:              int64((10 * time.Second).Seconds()),
	stageMaxInUnschedulablePodsDuration: 5 * time.Minute,
	parallelism:                         int32(16),
	// Ideally we would statically set the default profile here, but we can't because
	// creating the default profile may require testing feature gates, which may get
	// set dynamically in tests. Therefore, we delay creating it until New is actually
	// invoked.
	applyDefaultProfile: true,
}

// New returns a Scheduler
func New(client interface{},
	informerFactory interface{},
	dynInformerFactory interface{},
	recorderFactory interface{},
	stopCh <-chan struct{},
	opts ...Option) (*Scheduler, error) {

	stopEverything := stopCh
	if stopEverything == nil {
		stopEverything = NeverStop
	}

	options := defaultSchedulerOptions
	for _, opt := range opts {
		opt(&options)
	}

	if options.applyDefaultProfile {
		options.profiles = []config.Profile{
			config.Profile{
				SchedulerName: "default-scheduler",
				Plugins:       nil, // todo
				PluginConfig:  nil, // todo
			},
		}
	}

	registry := plugins.NewInTreeRegistry()
	if err := registry.Merge(options.frameworkOutOfTreeRegistry); err != nil {
		return nil, err
	}

	extender, err := buildExtenders(options.extenders, options.profiles)
	if err != nil {
		return nil, err
	}

	// todo podLister := informerFactory.Core().V1().Pods().Lister()
	// todo nodeLister := informerFactory.Core().V1().Nodes().Lister()

	profiles, err := profile.ne

}

func buildExtenders(extenders []config.Extender, profiles []config.Profile) ([]framework.Extender, error) {
	var fExtenders []framework.Extender
	if len(extenders) == 0 {
		return nil, nil
	}

	var ignoredExtendedResources []string
	var ignorableExtenders []framework.Extender
	for i := range extenders {
		flog.Infof("Creating extender %s %v", "extender", extenders[i])
		extender, err := NewHTTPExtender(&extenders[i])
		if err != nil {
			return nil, err
		}
		if !extender.IsIgnorable() {
			fExtenders = append(fExtenders, extender)
		} else {
			ignorableExtenders = append(ignorableExtenders, extender)
		}
		for _, r := range extenders[i].ManagedResources {
			ignoredExtendedResources = append(ignoredExtendedResources, r)
		}
	}
	// place ignorable extenders to the tail of extenders
	fExtenders = append(fExtenders, ignorableExtenders...)

	// If there are any extended resources found from the Extenders, append them to the pluginConfig for each profile.
	// This should only have an effect on ComponentConfig, where it is possible to configure Extenders and
	// plugin args (and in which case the extender ignored resources take precedence).
	if len(ignoredExtendedResources) == 0 {
		return fExtenders, nil
	}

	for i := range profiles {
		prof := &profiles[i]
		var found = false
		for k := range prof.PluginConfig {
			if prof.PluginConfig[k].Name == "something" { // todo
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("can't find NodeResourcesFitArgs in plugin config")
		}
	}
	return fExtenders, nil
}