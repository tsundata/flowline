package scheduler

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/informer/informers"
	listerv1 "github.com/tsundata/flowline/pkg/informer/listers/core/v1"
	"github.com/tsundata/flowline/pkg/scheduler/cache"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/scheduler/framework/config"
	"github.com/tsundata/flowline/pkg/scheduler/framework/plugins"
	frameworkruntime "github.com/tsundata/flowline/pkg/scheduler/framework/runtime"
	"github.com/tsundata/flowline/pkg/scheduler/profile"
	"github.com/tsundata/flowline/pkg/scheduler/queue"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"golang.org/x/xerrors"
	"sync"
	"sync/atomic"
	"time"
)

// ScheduleResult represents the result of scheduling a stage.
type ScheduleResult struct {
	// UID of the selected worker.
	SuggestedHost string
	// The number of workers the scheduler evaluated the stage against in the filtering
	// phase and beyond.
	EvaluatedWorkers int
	// The number of workers out of the evaluated ones that fit the stage.
	FeasibleWorkers int
}

func WithProfiles(p ...config.Profile) Option {
	return func(o *schedulerOptions) {
		o.profiles = p
		o.applyDefaultProfile = false
	}
}

func WithConfig(cfg *rest.Config) Option {
	return func(o *schedulerOptions) {
		o.config = cfg
	}
}

func WithPercentageOfWorkersToScore(percentageOfWorkersToScore int32) Option {
	return func(o *schedulerOptions) {
		o.percentageOfWorkersToScore = percentageOfWorkersToScore
	}
}

func WithFrameworkOutOfTreeRegistry(registry frameworkruntime.Registry) Option {
	return func(o *schedulerOptions) {
		o.frameworkOutOfTreeRegistry = registry
	}
}

func WithStageMaxBackoffSeconds(stageMaxBackoffSeconds int64) Option {
	return func(o *schedulerOptions) {
		o.stageMaxBackoffSeconds = stageMaxBackoffSeconds
	}
}

func WithStageInitialBackoffSeconds(stageInitialBackoffSeconds int64) Option {
	return func(o *schedulerOptions) {
		o.stageInitialBackoffSeconds = stageInitialBackoffSeconds
	}
}

func WithStageMaxInUnschedulableStagesDuration(duration time.Duration) Option {
	return func(o *schedulerOptions) {
		o.stageMaxInUnschedulableStagesDuration = duration
	}
}

func WithExtenders(e ...config.Extender) Option {
	return func(o *schedulerOptions) {
		o.extenders = e
	}
}

func WithParallelism(threads int32) Option {
	return func(o *schedulerOptions) {
		o.parallelism = threads
	}
}

func WithBuildFrameworkCapturer(fc FrameworkCapturer) Option {
	return func(o *schedulerOptions) {
		o.frameworkCapturer = fc
	}
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

	client client.Interface

	workerInfoSnapshot *cache.Snapshot

	percentageOfWorkersToScore int32
	nextStartWorkerIndex       int
}

type schedulerOptions struct {
	config                                *rest.Config
	percentageOfWorkersToScore            int32
	stageInitialBackoffSeconds            int64
	stageMaxBackoffSeconds                int64
	stageMaxInUnschedulableStagesDuration time.Duration
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

func (sched *Scheduler) Run(ctx context.Context) {
	sched.SchedulingQueue.Run()

	go parallelizer.JitterUntilWithContext(ctx, sched.scheduleOne, 0, 0.0, true)

	<-ctx.Done()
	flog.Info("SchedulingQueue close")
	sched.SchedulingQueue.Close()
}

func (sched *Scheduler) scheduleStage(ctx context.Context, fwk framework.Framework, state *framework.CycleState, stage *meta.Stage) (result ScheduleResult, err error) {
	flog.Infof("scheduleStage %s %s", stage.Name, stage.UID)
	if err := sched.Cache.UpdateSnapshot(sched.workerInfoSnapshot); err != nil {
		return result, err
	}

	if sched.workerInfoSnapshot.NumWorkers() == 0 {
		return result, xerrors.New("ErrNoWorkersAvailable")
	}

	feasibleWorkers, diagnosis, err := sched.findWorkersThatFitStage(ctx, fwk, state, stage)
	if err != nil {
		return result, err
	}

	if len(feasibleWorkers) == 0 {
		return result, xerrors.New("FitError")
	}

	// When only one worker after predicate, just use it.
	if len(feasibleWorkers) == 1 {
		return ScheduleResult{
			SuggestedHost:    feasibleWorkers[0].UID,
			EvaluatedWorkers: 1 + len(diagnosis.WorkerToStatusMap),
			FeasibleWorkers:  1,
		}, nil
	}

	priorityList, err := prioritizeWorkers(ctx, sched.Extenders, fwk, state, stage, feasibleWorkers)
	if err != nil {
		return result, err
	}

	host, err := selectHost(priorityList)

	return ScheduleResult{
		SuggestedHost:    host,
		EvaluatedWorkers: len(feasibleWorkers) + len(diagnosis.WorkerToStatusMap),
		FeasibleWorkers:  len(feasibleWorkers),
	}, err
}

// Filters the workers to find the ones that fit the stage based on the framework
// filter plugins and filter extenders.
func (sched *Scheduler) findWorkersThatFitStage(ctx context.Context, fwk framework.Framework, state *framework.CycleState, stage *meta.Stage) ([]*meta.Worker, framework.Diagnosis, error) {
	diagnosis := framework.Diagnosis{
		WorkerToStatusMap:    make(framework.WorkerToStatusMap),
		UnschedulablePlugins: make(map[string]struct{}),
	}

	allWorkers, err := sched.workerInfoSnapshot.WorkerInfos().List()
	if err != nil {
		return nil, diagnosis, err
	}

	workers := allWorkers
	feasibleWorkers, err := sched.findWorkersThatPassFilters(ctx, fwk, state, stage, diagnosis, workers)
	processedWorkers := len(feasibleWorkers) + len(diagnosis.WorkerToStatusMap)
	sched.nextStartWorkerIndex = (sched.nextStartWorkerIndex + processedWorkers) % len(workers)
	if err != nil {
		return nil, diagnosis, err
	}

	feasibleWorkers, err = findWorkersThatPassExtenders(sched.Extenders, stage, feasibleWorkers, diagnosis.WorkerToStatusMap)
	if err != nil {
		return nil, diagnosis, err
	}
	return feasibleWorkers, diagnosis, nil
}

// findWorkersThatPassFilters finds the workers that fit the filter plugins.
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
		status := fwk.RunFilterPluginsWithNominatedStages(ctx, state, stage, workerInfo)
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

func findWorkersThatPassExtenders(extenders []framework.Extender, stage *meta.Stage, feasibleWorkers []*meta.Worker, statuses framework.WorkerToStatusMap) ([]*meta.Worker, error) {
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
	client client.Interface,
	workerInfoSnapshot *cache.Snapshot,
	percentageOfWorkersToScore int32) *Scheduler {
	sched := Scheduler{
		Cache:                      cache,
		Extenders:                  extenders,
		NextStage:                  nextStage,
		Error:                      Error,
		StopEverything:             stopEverything,
		SchedulingQueue:            schedulingQueue,
		Profiles:                   profiles,
		client:                     client,
		workerInfoSnapshot:         workerInfoSnapshot,
		percentageOfWorkersToScore: percentageOfWorkersToScore,
	}
	sched.ScheduleStage = sched.scheduleStage
	return &sched
}

// NeverStop may be passed to Until to make it never stop.
var NeverStop <-chan struct{} = make(chan struct{})

var defaultSchedulerOptions = schedulerOptions{
	percentageOfWorkersToScore:            0,
	stageInitialBackoffSeconds:            int64((1 * time.Second).Seconds()),
	stageMaxBackoffSeconds:                int64((10 * time.Second).Seconds()),
	stageMaxInUnschedulableStagesDuration: 5 * time.Minute,
	parallelism:                           int32(16),
	// Ideally we would statically set the default profile here, but we can't because
	// creating the default profile may require testing feature gates, which may get
	// set dynamically in tests. Therefore, we delay creating it until New is actually
	// invoked.
	applyDefaultProfile: true,
}

// New returns a Scheduler
func New(client client.Interface,
	informerFactory informers.SharedInformerFactory,
	recorderFactory profile.RecorderFactory,
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
		options.profiles = plugins.DefaultProfile
	}

	registry := plugins.NewInTreeRegistry()
	if err := registry.Merge(options.frameworkOutOfTreeRegistry); err != nil {
		return nil, err
	}

	extenders, err := buildExtenders(options.extenders, options.profiles)
	if err != nil {
		return nil, err
	}

	snapshot := cache.NewEmptySnapshot()

	stageLister := informerFactory.Core().V1().Stages().Lister()
	nominator := queue.NewStageNominator(stageLister)

	profiles, err := profile.NewMap(options.profiles, registry, recorderFactory, stopCh,
		frameworkruntime.WithExtenders(extenders),
		frameworkruntime.WithStageNominator(nominator),
		frameworkruntime.WithClientSet(client),
	)
	if err != nil {
		return nil, err
	}

	stageQueue := queue.NewSchedulingQueue(
		profiles[options.profiles[0].SchedulerName].QueueSortFunc(),
		informerFactory,
	)

	schedulerCache := cache.New(0, stopEverything)

	sched := newScheduler(
		schedulerCache,
		extenders,
		queue.MakeNextStageFunc(stageQueue),
		MakeDefaultErrorFunc(client, stageLister, stageQueue, schedulerCache),
		stopEverything,
		stageQueue,
		profiles,
		client,
		snapshot,
		options.percentageOfWorkersToScore,
	)

	addAllEventHandlers(sched, informerFactory, nil)

	return sched, nil
}

func MakeDefaultErrorFunc(client client.Interface, stageLister listerv1.StageLister, stageQueue queue.SchedulingQueue, schedulerCache cache.Cache) func(*framework.QueuedStageInfo, error) {
	return func(stageInfo *framework.QueuedStageInfo, err error) {
		stage := stageInfo.Stage
		if err == xerrors.New("IsNotFound") { // todo
			workerUID := "something uid"
			_, err := client.CoreV1().Worker().Get(context.Background(), workerUID, meta.GetOptions{})
			if err != nil {
				worker := meta.Worker{ObjectMeta: meta.ObjectMeta{UID: workerUID}}
				if err := schedulerCache.RemoveWorker(&worker); err != nil {
					flog.Errorf("Worker is not found; failed to remove it from the cache %s", worker.UID)
				}
			}
		} else {
			flog.Errorf("%s Error scheduling stage; retrying %s %s", err, stage.Name, stage.UID)
		}

		cachedStage, err := stageLister.Get(stage.UID)
		if err != nil {
			flog.Errorf("Stage doesn't exist in informer cache %s %s", stage.UID, err)
			return
		}

		// As <cachedStage> is from SharedInformer, we need to do a DeepCopy() here.
		stageInfo.StageInfo = framework.NewStageInfo(cachedStage)
		if err := stageQueue.AddUnschedulableIfNotPresent(stageInfo, stageQueue.SchedulingCycle()); err != nil {
			flog.Error(err)
		}
	}
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
		ignoredExtendedResources = append(ignoredExtendedResources, extenders[i].ManagedResources...)
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
			return nil, xerrors.Errorf("can't find WorkerResourcesFitArgs in plugin config")
		}
	}
	return fExtenders, nil
}
