package runtime

import (
	"context"
	"fmt"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/scheduler/framework/config"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"reflect"
	"sort"
	"sync"
	"time"
)

const (
	// Specifies the maximum timeout a permit plugin can return.
	maxTimeout = 15 * time.Minute
)

type frameworkOptions struct {
	clientSet       client.Interface
	config          interface{}
	eventRecorder   interface{}
	informerFactory interface{}
	// snapshotSharedLister   interface{}
	stageNominator  framework.StageNominator
	extenders       []framework.Extender
	captureProfile  CaptureProfile
	clusterEventMap map[framework.ClusterEvent]map[string]struct{}
	parallelizer    parallelizer.Parallelizer
}

// Option for the frameworkImpl.
type Option func(*frameworkOptions)

// CaptureProfile is a callback to capture a finalized profile.
type CaptureProfile func(config.Profile)

// WithEventRecorder sets clientSet for the scheduling frameworkImpl.
func WithEventRecorder(recorder interface{}) Option {
	return func(o *frameworkOptions) {
		o.eventRecorder = recorder
	}
}

// WithStageNominator sets stageNominator for the scheduling frameworkImpl.
func WithStageNominator(nominator framework.StageNominator) Option {
	return func(o *frameworkOptions) {
		o.stageNominator = nominator
	}
}

// WithExtenders sets extenders for the scheduling frameworkImpl.
func WithExtenders(extenders []framework.Extender) Option {
	return func(o *frameworkOptions) {
		o.extenders = extenders
	}
}

// WithClientSet sets clientSet for the scheduling frameworkImpl.
func WithClientSet(clientSet client.Interface) Option {
	return func(o *frameworkOptions) {
		o.clientSet = clientSet
	}
}

func defaultFrameworkOptions(_ <-chan struct{}) frameworkOptions {
	return frameworkOptions{
		clusterEventMap: make(map[framework.ClusterEvent]map[string]struct{}),
		parallelizer:    parallelizer.NewParallelizer(parallelizer.DefaultParallelism),
	}
}

// NewFramework initializes plugins given the configuration and the registry.
func NewFramework(r Registry, profile *config.Profile, stopCh <-chan struct{}, opts ...Option) (framework.Framework, error) {
	options := defaultFrameworkOptions(stopCh)
	for _, opt := range opts {
		opt(&options)
	}

	f := &frameworkImpl{
		registry:          r,
		scorePluginWeight: make(map[string]int),
		waitingStages:     newWaitingStagesMap(),
		clientSet:         options.clientSet,
		config:            options.config,
		eventRecorder:     options.eventRecorder,
		informerFactory:   options.informerFactory,
		extenders:         options.extenders,
		StageNominator:    options.stageNominator,
		parallelizer:      options.parallelizer,
	}

	if profile == nil {
		return f, nil
	}

	f.profileName = profile.SchedulerName
	if profile.Plugins == nil {
		return f, nil
	}

	// get needed plugins from config
	pg := f.pluginsNeeded(profile.Plugins)

	pluginConfig := make(map[string]runtime.Object, len(profile.PluginConfig))
	for i := range profile.PluginConfig {
		name := profile.PluginConfig[i].Name
		if _, ok := pluginConfig[name]; ok {
			return nil, fmt.Errorf("repeated config for plugin %s", name)
		}
		pluginConfig[name] = profile.PluginConfig[i].Args
	}
	outputProfile := config.Profile{
		SchedulerName: f.profileName,
		Plugins:       profile.Plugins,
		PluginConfig:  make([]config.PluginConfig, 0, len(pg)),
	}

	pluginsMap := make(map[string]framework.Plugin)
	for name, factory := range r {
		// initialize only needed plugins.
		if _, ok := pg[name]; !ok {
			continue
		}

		args := pluginConfig[name]
		if args != nil {
			outputProfile.PluginConfig = append(outputProfile.PluginConfig, config.PluginConfig{
				Name: name,
				Args: args,
			})
		}
		p, err := factory(args, f)
		if err != nil {
			return nil, fmt.Errorf("initializing plugin %q: %w", name, err)
		}
		pluginsMap[name] = p

		// Update ClusterEventMap in place.
		fillEventToPluginMap(p, options.clusterEventMap)
	}

	// initialize plugins per individual extension points
	for _, e := range f.getExtensionPoints(profile.Plugins) {
		if err := updatePluginList(e.slicePtr, *e.plugins, pluginsMap); err != nil {
			return nil, err
		}
	}

	if len(f.queueSortPlugins) != 1 {
		return nil, fmt.Errorf("only one queue sort plugin required for profile with scheduler name %q, but got %d", profile.SchedulerName, len(f.queueSortPlugins))
	}
	if len(f.bindPlugins) == 0 {
		return nil, fmt.Errorf("at least one bind plugin is needed for profile with scheduler name %q", profile.SchedulerName)
	}

	if err := getScoreWeights(f, pluginsMap, profile.Plugins.Score.Enabled); err != nil {
		return nil, err
	}

	// Verifying the score weights again since Plugin.Name() could return a different
	// value from the one used in the configuration.
	for _, scorePlugin := range f.scorePlugins {
		if f.scorePluginWeight[scorePlugin.Name()] == 0 {
			return nil, fmt.Errorf("score plugin %q is not configured with weight", scorePlugin.Name())
		}
	}

	if options.captureProfile != nil {
		if len(outputProfile.PluginConfig) != 0 {
			sort.Slice(outputProfile.PluginConfig, func(i, j int) bool {
				return outputProfile.PluginConfig[i].Name < outputProfile.PluginConfig[j].Name
			})
		} else {
			outputProfile.PluginConfig = nil
		}
		options.captureProfile(outputProfile)
	}

	return f, nil
}

// waitingStagesMap a thread-safe map used to maintain stages waiting in the permit phase.
type waitingStagesMap struct {
	stages map[string]*waitingStage
	mu     sync.RWMutex
}

// newWaitingStagesMap returns a new waitingStagesMap.
func newWaitingStagesMap() *waitingStagesMap {
	return &waitingStagesMap{
		stages: make(map[string]*waitingStage),
	}
}

// add a new WaitingStage to the map.
func (m *waitingStagesMap) add(wp *waitingStage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stages[wp.GetStage().UID] = wp
}

// remove a WaitingStage from the map.
func (m *waitingStagesMap) remove(uid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.stages, uid)
}

// get a WaitingStage from the map.
func (m *waitingStagesMap) get(uid string) *waitingStage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stages[uid]
}

// iterate acquires a read lock and iterates over the WaitingStage map.
func (m *waitingStagesMap) iterate(callback func(framework.WaitingStage)) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, v := range m.stages {
		callback(v)
	}
}

// waitingStage represents a stage waiting in the permit phase.
type waitingStage struct {
	stage          *meta.Stage
	pendingPlugins map[string]*time.Timer
	s              chan *framework.Status
	mu             sync.RWMutex
}

var _ framework.WaitingStage = &waitingStage{}

// newWaitingStage returns a new waitingStage instance.
func newWaitingStage(stage *meta.Stage, pluginsMaxWaitTime map[string]time.Duration) *waitingStage {
	wp := &waitingStage{
		stage: stage,
		// Allow() and Reject() calls are non-blocking. This property is guaranteed
		// by using non-blocking send to this channel. This channel has a buffer of size 1
		// to ensure that non-blocking send will not be ignored - possible situation when
		// receiving from this channel happens after non-blocking send.
		s: make(chan *framework.Status, 1),
	}

	wp.pendingPlugins = make(map[string]*time.Timer, len(pluginsMaxWaitTime))
	// The time.AfterFunc calls wp.Reject which iterates through pendingPlugins map. Acquire the
	// lock here so that time.AfterFunc can only execute after newWaitingStage finishes.
	wp.mu.Lock()
	defer wp.mu.Unlock()
	for k, v := range pluginsMaxWaitTime {
		plugin, waitTime := k, v
		wp.pendingPlugins[plugin] = time.AfterFunc(waitTime, func() {
			msg := fmt.Sprintf("rejected due to timeout after waiting %v at plugin %v",
				waitTime, plugin)
			wp.Reject(plugin, msg)
		})
	}

	return wp
}

func (w *waitingStage) GetStage() *meta.Stage {
	return w.stage
}

func (w *waitingStage) GetPendingPlugins() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	plugins := make([]string, 0, len(w.pendingPlugins))
	for p := range w.pendingPlugins {
		plugins = append(plugins, p)
	}

	return plugins
}

func (w *waitingStage) Allow(pluginName string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if timer, exist := w.pendingPlugins[pluginName]; exist {
		timer.Stop()
		delete(w.pendingPlugins, pluginName)
	}

	// Only signal success status after all plugins have allowed
	if len(w.pendingPlugins) != 0 {
		return
	}

	// The select clause works as a non-blocking send.
	// If there is no receiver, it's a no-op (default case).
	select {
	case w.s <- framework.NewStatus(framework.Success, ""):
	default:
	}
}

func (w *waitingStage) Reject(pluginName, msg string) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, timer := range w.pendingPlugins {
		timer.Stop()
	}

	// The select clause works as a non-blocking send.
	// If there is no receiver, it's a no-op (default case).
	select {
	case w.s <- framework.NewStatus(framework.Unschedulable, msg).WithFailedPlugin(pluginName):
	default:
	}
}

// frameworkImpl is the component responsible for initializing and running scheduler
// plugins.
type frameworkImpl struct {
	registry          Registry
	waitingStages     *waitingStagesMap
	scorePluginWeight map[string]int
	queueSortPlugins  []framework.QueueSortPlugin
	filterPlugins     []framework.FilterPlugin
	scorePlugins      []framework.ScorePlugin
	bindPlugins       []framework.BindPlugin
	permitPlugins     []framework.PermitPlugin

	clientSet       client.Interface
	config          interface{}
	eventRecorder   interface{}
	informerFactory interface{}

	profileName string

	extenders []framework.Extender
	framework.StageNominator

	parallelizer parallelizer.Parallelizer
}

func (f *frameworkImpl) RunScorePlugins(ctx context.Context, state *framework.CycleState, stage *meta.Stage, workers []*meta.Worker) (framework.PluginToWorkerScores, *framework.Status) {
	pluginToWorkerScores := make(framework.PluginToWorkerScores, len(f.scorePlugins))
	for _, pl := range f.scorePlugins {
		pluginToWorkerScores[pl.Name()] = make(framework.WorkerScoreList, len(workers))
	}
	ctx, cancel := context.WithCancel(ctx)
	errCh := parallelizer.NewErrorChannel()

	// Run Score method for each worker in parallel.
	f.Parallelizer().Until(ctx, len(workers), func(index int) {
		for _, pl := range f.scorePlugins {
			workerName := workers[index].UID
			s, ss := f.runScorePlugin(ctx, pl, state, stage, workerName)
			if !ss.IsSuccess() {
				err := fmt.Errorf("plugin %q failed with: %w", pl.Name(), ss.AsError())
				errCh.SendErrorWithCancel(err, cancel)
				return
			}
			pluginToWorkerScores[pl.Name()][index] = framework.WorkerScore{
				UID:   workerName,
				Score: s,
			}
		}
	})
	if err := errCh.ReceiveError(); err != nil {
		return nil, framework.AsStatus(fmt.Errorf("running Score plugins: %w", err))
	}

	// Run NormalizeScore method for each ScorePlugin in parallel.
	f.Parallelizer().Until(ctx, len(f.scorePlugins), func(index int) {
		pl := f.scorePlugins[index]
		workerScoreList := pluginToWorkerScores[pl.Name()]
		if pl.ScoreExtensions() == nil {
			return
		}
		ss := f.runScoreExtension(ctx, pl, state, stage, workerScoreList)
		if !ss.IsSuccess() {
			err := fmt.Errorf("plugin %q failed with: %w", pl.Name(), ss.AsError())
			errCh.SendErrorWithCancel(err, cancel)
			return
		}
	})
	if err := errCh.ReceiveError(); err != nil {
		return nil, framework.AsStatus(fmt.Errorf("running Normalize on Score plugins: %w", err))
	}

	// Apply score defaultWeights for each ScorePlugin in parallel.
	f.Parallelizer().Until(ctx, len(f.scorePlugins), func(index int) {
		pl := f.scorePlugins[index]
		// Score plugins' weight has been checked when they are initialized.
		weight := f.scorePluginWeight[pl.Name()]
		workerScoreList := pluginToWorkerScores[pl.Name()]

		for i, workerScore := range workerScoreList {
			// return error if score plugin returns invalid score.
			if workerScore.Score > framework.MaxWorkerScore || workerScore.Score < framework.MinWorkerScore {
				err := fmt.Errorf("plugin %q returns an invalid score %v, it should in the range of [%v, %v] after normalizing", pl.Name(), workerScore.Score, framework.MinWorkerScore, framework.MaxWorkerScore)
				errCh.SendErrorWithCancel(err, cancel)
				return
			}
			workerScoreList[i].Score = workerScore.Score * int64(weight)
		}
	})
	if err := errCh.ReceiveError(); err != nil {
		return nil, framework.AsStatus(fmt.Errorf("applying score defaultWeights on Score plugins: %w", err))
	}

	return pluginToWorkerScores, nil
}

func (f *frameworkImpl) runScorePlugin(ctx context.Context, pl framework.ScorePlugin, state *framework.CycleState, stage *meta.Stage, workerName string) (int64, *framework.Status) {
	if !state.ShouldRecordPluginMetrics() {
		return pl.Score(ctx, state, stage, workerName)
	}
	s, ss := pl.Score(ctx, state, stage, workerName)
	return s, ss
}

func (f *frameworkImpl) runScoreExtension(ctx context.Context, pl framework.ScorePlugin, state *framework.CycleState, stage *meta.Stage, workerScoreList framework.WorkerScoreList) *framework.Status {
	if !state.ShouldRecordPluginMetrics() {
		return pl.ScoreExtensions().NormalizeScore(ctx, state, stage, workerScoreList)
	}
	ss := pl.ScoreExtensions().NormalizeScore(ctx, state, stage, workerScoreList)
	return ss
}

func (f *frameworkImpl) RunFilterPlugins(ctx context.Context, state *framework.CycleState, stage *meta.Stage, info *framework.WorkerInfo) framework.PluginToStatus {
	statuses := make(framework.PluginToStatus)
	for _, pl := range f.filterPlugins {
		pluginStatus := f.runFilterPlugin(ctx, pl, state, stage, info)
		if !pluginStatus.IsSuccess() {
			if !pluginStatus.IsUnschedulable() {
				// Filter plugins are not supposed to return any status other than
				// Success or Unschedulable.
				errStatus := framework.AsStatus(fmt.Errorf("running %q filter plugin: %w", pl.Name(), pluginStatus.AsError())).WithFailedPlugin(pl.Name())
				return map[string]*framework.Status{pl.Name(): errStatus}
			}
			pluginStatus.SetFailedPlugin(pl.Name())
			statuses[pl.Name()] = pluginStatus
		}
	}

	return statuses
}

func (f *frameworkImpl) runFilterPlugin(ctx context.Context, pl framework.FilterPlugin, state *framework.CycleState, stage *meta.Stage, workerInfo *framework.WorkerInfo) *framework.Status {
	if !state.ShouldRecordPluginMetrics() {
		return pl.Filter(ctx, state, stage, workerInfo)
	}
	ss := pl.Filter(ctx, state, stage, workerInfo)
	return ss
}

func (f *frameworkImpl) IterateOverWaitingStages(callback func(framework.WaitingStage)) {
	f.waitingStages.iterate(callback)
}

func (f *frameworkImpl) GetWaitingStage(uid string) framework.WaitingStage {
	if wp := f.waitingStages.get(uid); wp != nil {
		return wp
	}
	return nil // Returning nil instead of *waitingStage(nil).
}

func (f *frameworkImpl) RejectWaitingStage(uid string) bool {
	if waitingStage := f.waitingStages.get(uid); waitingStage != nil {
		waitingStage.Reject("", "removed")
		return true
	}
	return false
}

func (f *frameworkImpl) ClientSet() client.Interface {
	return f.clientSet
}

func (f *frameworkImpl) EventRecorder() interface{} {
	return f.eventRecorder
}

func (f *frameworkImpl) RunFilterPluginsWithNominatedStages(ctx context.Context, state *framework.CycleState, stage *meta.Stage, info *framework.WorkerInfo) *framework.Status {
	var status *framework.Status

	stagesAdded := false

	for i := 0; i < 2; i++ {
		stateToUse := state
		workerInfoToUse := info
		if i == 0 {
			var err error
			stagesAdded, stateToUse, workerInfoToUse, err = addNominatedStages(ctx, f, stage, state, info)
			if err != nil {
				return framework.AsStatus(err)
			}
		} else if !stagesAdded || !status.IsSuccess() {
			break
		}

		statusMap := f.RunFilterPlugins(ctx, stateToUse, stage, workerInfoToUse)
		status = statusMap.Merge()
		if !status.IsSuccess() && !status.IsUnschedulable() {
			return status
		}
	}

	return status
}

// addNominatedStages adds stages with equal or greater priority which are nominated
// to run on the worker. It returns 1) whether any stage was added, 2) augmented cycleState,
// 3) augmented workerInfo.
func addNominatedStages(_ context.Context, fh framework.Handle, stage *meta.Stage, state *framework.CycleState, workerInfo *framework.WorkerInfo) (bool, *framework.CycleState, *framework.WorkerInfo, error) {
	if fh == nil || workerInfo.Worker() == nil {
		// This may happen only in tests.
		return false, state, workerInfo, nil
	}
	nominatedStageInfos := fh.NominatedStagesForWorker(workerInfo.Worker().Name)
	if len(nominatedStageInfos) == 0 {
		return false, state, workerInfo, nil
	}
	workerInfoOut := workerInfo.Clone()
	stateOut := state.Clone()
	stagesAdded := false
	for _, pi := range nominatedStageInfos {
		if StagePriority(pi.Stage) >= StagePriority(stage) && pi.Stage.UID != stage.UID {
			workerInfoOut.AddStageInfo(pi)
			stagesAdded = true
		}
	}
	return stagesAdded, stateOut, workerInfoOut, nil
}

func StagePriority(stage *meta.Stage) int {
	if stage.Priority != 0 {
		return stage.Priority
	}
	// When priority of a running stage is nil, it means it was created at a time
	// that there was no global default priority class and the priority class
	// name of the stage was empty. So, we resolve to the static default priority.
	return 0
}

func (f *frameworkImpl) Extenders() []framework.Extender {
	return f.extenders
}

func (f *frameworkImpl) Parallelizer() parallelizer.Parallelizer {
	return f.parallelizer
}

func (f *frameworkImpl) QueueSortFunc() framework.LessFunc {
	if f == nil {
		return func(_, _ *framework.QueuedStageInfo) bool {
			return false
		}
	}

	if len(f.queueSortPlugins) == 0 {
		panic("No QueueSort plugin is registered in the frameworkImpl.")
	}

	return f.queueSortPlugins[0].Less
}

func (f *frameworkImpl) RunPermitPlugins(ctx context.Context, state *framework.CycleState, stage *meta.Stage, workerUID string) *framework.Status {
	pluginsWaitTime := make(map[string]time.Duration)
	statusCode := framework.Success
	for _, pl := range f.permitPlugins {
		status, timeout := f.runPermitPlugin(ctx, pl, state, stage, workerUID)
		if !status.IsSuccess() {
			if status.IsUnschedulable() {
				flog.Infof("Stage rejected by permit plugin %v %s %s", stage, pl.Name(), status.Message())
				status.SetFailedPlugin(pl.Name())
				return status
			}
			if status.IsWait() {
				// Not allowed to be greater than maxTimeout.
				if timeout > maxTimeout {
					timeout = maxTimeout
				}
				pluginsWaitTime[pl.Name()] = timeout
				statusCode = framework.Wait
			} else {
				err := status.AsError()
				flog.Error(err)
				flog.Errorf("Failed running Permit plugin %s %v", pl.Name(), stage)
				return framework.AsStatus(fmt.Errorf("running Permit plugin %q: %w", pl.Name(), err)).WithFailedPlugin(pl.Name())
			}
		}
	}
	if statusCode == framework.Wait {
		waitingStage := newWaitingStage(stage, pluginsWaitTime)
		f.waitingStages.add(waitingStage)
		msg := fmt.Sprintf("one or more plugins asked to wait and no plugin rejected stage %q", stage.Name)
		flog.Infof("One or more plugins asked to wait and no plugin rejected stage %v", stage)
		return framework.NewStatus(framework.Wait, msg)
	}
	return nil
}

func (f *frameworkImpl) runPermitPlugin(ctx context.Context, pl framework.PermitPlugin, state *framework.CycleState, stage *meta.Stage, workerUID string) (*framework.Status, time.Duration) {
	if !state.ShouldRecordPluginMetrics() {
		return pl.Permit(ctx, state, stage, workerUID)
	}
	ss, timeout := pl.Permit(ctx, state, stage, workerUID)
	return ss, timeout
}

func (f *frameworkImpl) WaitOnPermit(_ context.Context, stage *meta.Stage) *framework.Status {
	waitingStage := f.waitingStages.get(stage.UID)
	if waitingStage == nil {
		return nil
	}
	defer f.waitingStages.remove(stage.UID)
	flog.Infof("Stage waiting on permit %v", stage)

	s := <-waitingStage.s

	if !s.IsSuccess() {
		if s.IsUnschedulable() {
			flog.Infof("Stage rejected while waiting on permit, %v %s", stage, s.Message())
			s.SetFailedPlugin(s.FailedPlugin())
			return s
		}
		err := s.AsError()
		flog.Error(err)
		flog.Errorf("Failed waiting on permit for stage, %v", stage)
		return framework.AsStatus(fmt.Errorf("waiting on permit for stage: %w", err)).WithFailedPlugin(s.FailedPlugin())
	}
	return nil
}

func (f *frameworkImpl) RunBindPlugins(ctx context.Context, state *framework.CycleState, stage *meta.Stage, workerUID string) *framework.Status {
	if len(f.bindPlugins) == 0 {
		return framework.NewStatus(framework.Skip, "")
	}
	var status *framework.Status
	for _, bp := range f.bindPlugins {
		status = f.runBindPlugin(ctx, bp, state, stage, workerUID)
		if status.IsSkip() {
			continue
		}
		if !status.IsSuccess() {
			err := status.AsError()
			flog.Error(err)
			flog.Errorf("Failed running Bind plugin, %s %v", bp.Name(), stage)
			return framework.AsStatus(fmt.Errorf("running Bind plugin %q: %w", bp.Name(), err))
		}
		return status
	}
	return status
}

func (f *frameworkImpl) runBindPlugin(ctx context.Context, bp framework.BindPlugin, state *framework.CycleState, stage *meta.Stage, workerUID string) *framework.Status {
	if !state.ShouldRecordPluginMetrics() {
		return bp.Bind(ctx, state, stage, workerUID)
	}
	ss := bp.Bind(ctx, state, stage, workerUID)
	return ss
}

func (f *frameworkImpl) HasFilterPlugins() bool {
	return len(f.filterPlugins) > 0
}

func (f *frameworkImpl) HasScorePlugins() bool {
	return len(f.scorePlugins) > 0
}

func (f *frameworkImpl) ListPlugins() *config.Plugins {
	m := config.Plugins{}

	for _, e := range f.getExtensionPoints(&m) {
		plugins := reflect.ValueOf(e.slicePtr).Elem()
		extName := plugins.Type().Elem().Name()
		var cfgs []config.Plugin
		for i := 0; i < plugins.Len(); i++ {
			name := plugins.Index(i).Interface().(framework.Plugin).Name()
			p := config.Plugin{Name: name}
			if extName == "ScorePlugin" {
				// Weights apply only to score plugins.
				p.Weight = int32(f.scorePluginWeight[name])
			}
			cfgs = append(cfgs, p)
		}
		if len(cfgs) > 0 {
			e.plugins.Enabled = cfgs
		}
	}
	return &m
}

func (f *frameworkImpl) ProfileName() string {
	return f.profileName
}

func (f *frameworkImpl) pluginsNeeded(plugins *config.Plugins) map[string]struct{} {
	pgSet := make(map[string]struct{})

	if plugins == nil {
		return pgSet
	}

	find := func(pgs *config.PluginSet) {
		for _, pg := range pgs.Enabled {
			pgSet[pg.Name] = struct{}{}
		}
	}

	for _, e := range f.getExtensionPoints(plugins) {
		find(e.plugins)
	}

	return pgSet
}

// extensionPoint encapsulates desired and applied set of plugins at a specific extension
// point. This is used to simplify iterating over all extension points supported by the
// frameworkImpl.
type extensionPoint struct {
	// the set of plugins to be configured at this extension point.
	plugins *config.PluginSet
	// a pointer to the slice storing plugins implementations that will run at this
	// extension point.
	slicePtr interface{}
}

func (f *frameworkImpl) getExtensionPoints(plugins *config.Plugins) []extensionPoint {
	return []extensionPoint{
		{&plugins.Filter, &f.filterPlugins},
		{&plugins.Score, &f.scorePlugins},
		{&plugins.Bind, &f.bindPlugins},
		{&plugins.Permit, &f.permitPlugins},
		{&plugins.QueueSort, &f.queueSortPlugins},
	}
}

var allClusterEvents = []framework.ClusterEvent{
	{Resource: framework.Stage, ActionType: framework.All},
	{Resource: framework.Worker, ActionType: framework.All},
}

func fillEventToPluginMap(p framework.Plugin, eventToPlugins map[framework.ClusterEvent]map[string]struct{}) {
	ext, ok := p.(framework.EnqueueExtensions)
	if !ok {
		// If interface EnqueueExtensions is not implemented, register the default events
		// to the plugin. This is to ensure backward compatibility.
		registerClusterEvents(p.Name(), eventToPlugins, allClusterEvents)
		return
	}

	events := ext.EventsToRegister()
	// It's rare that a plugin implements EnqueueExtensions but returns nil.
	// We treat it as: the plugin is not interested in any event, and hence stage failed by that plugin
	// cannot be moved by any regular cluster event.
	if len(events) == 0 {
		flog.Infof("Plugin's EventsToRegister() returned nil %s", p.Name())
		return
	}
	// The most common case: a plugin implements EnqueueExtensions and returns non-nil result.
	registerClusterEvents(p.Name(), eventToPlugins, events)
}

func registerClusterEvents(name string, eventToPlugins map[framework.ClusterEvent]map[string]struct{}, evts []framework.ClusterEvent) {
	for _, evt := range evts {
		if eventToPlugins[evt] == nil {
			eventToPlugins[evt] = map[string]struct{}{name: {}}
		} else {
			eventToPlugins[evt][name] = struct{}{}
		}
	}
}

func updatePluginList(pluginList interface{}, pluginSet config.PluginSet, pluginsMap map[string]framework.Plugin) error {
	plugins := reflect.ValueOf(pluginList).Elem()
	pluginType := plugins.Type().Elem()
	set := make(map[string]struct{})
	for _, ep := range pluginSet.Enabled {
		pg, ok := pluginsMap[ep.Name]
		if !ok {
			return fmt.Errorf("%s %q does not exist", pluginType.Name(), ep.Name)
		}

		if !reflect.TypeOf(pg).Implements(pluginType) {
			return fmt.Errorf("plugin %q does not extend %s plugin", ep.Name, pluginType.Name())
		}

		if _, ok := set[ep.Name]; ok {
			return fmt.Errorf("plugin %q already registered as %q", ep.Name, pluginType.Name())
		}

		set[ep.Name] = struct{}{}

		newPlugins := reflect.Append(plugins, reflect.ValueOf(pg))
		plugins.Set(newPlugins)
	}
	return nil
}

// getScoreWeights makes sure that, between MultiPoint-Score plugin weights and individual Score
// plugin weights there is not an overflow of MaxTotalScore.
func getScoreWeights(f *frameworkImpl, pluginsMap map[string]framework.Plugin, plugins []config.Plugin) error {
	var totalPriority int64
	scorePlugins := reflect.ValueOf(&f.scorePlugins).Elem()
	pluginType := scorePlugins.Type().Elem()
	for _, e := range plugins {
		pg := pluginsMap[e.Name]
		if !reflect.TypeOf(pg).Implements(pluginType) {
			continue
		}

		// We append MultiPoint plugins to the list of Score plugins. So if this plugin has already been
		// encountered, let the individual Score weight take precedence.
		if _, ok := f.scorePluginWeight[e.Name]; ok {
			continue
		}
		// a weight of zero is not permitted, plugins can be disabled explicitly
		// when configured.
		f.scorePluginWeight[e.Name] = int(e.Weight)
		if f.scorePluginWeight[e.Name] == 0 {
			f.scorePluginWeight[e.Name] = 1
		}

		// Checks totalPriority against MaxTotalScore to avoid overflow
		if int64(f.scorePluginWeight[e.Name])*framework.MaxWorkerScore > framework.MaxTotalScore-totalPriority {
			return fmt.Errorf("total score of Score plugins could overflow")
		}
		totalPriority += int64(f.scorePluginWeight[e.Name]) * framework.MaxWorkerScore
	}
	return nil
}
