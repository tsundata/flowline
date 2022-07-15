package runtime

import (
	"context"
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/scheduler/framework/config"
	"sort"
	"sync"
	"time"
)

type frameworkOptions struct {
	componentConfigVersion string
	clientSet              interface{}
	config                 interface{}
	eventRecorder          interface{}
	informerFactory        interface{}
	snapshotSharedLister   interface{}
	stageNominator         framework.StageNominator
	extenders              []framework.Extender
	captureProfile         CaptureProfile
	clusterEventMap        map[framework.ClusterEvent]map[string]struct{}
	parallelizer           int
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

func defaultFrameworkOptions(stopCh <-chan struct{}) frameworkOptions {
	return frameworkOptions{
		clusterEventMap: make(map[framework.ClusterEvent]map[string]struct{}),
		parallelizer:    16,
	}
}

// newWaitingPodsMap returns a new waitingPodsMap.
func newWaitingPodsMap() *waitingStagesMap {
	return &waitingStagesMap{
		stages: make(map[string]*waitingStage),
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
		waitingPods:       newWaitingPodsMap(),
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
	outputProfile := config.KubeSchedulerProfile{
		SchedulerName: f.profileName,
		Plugins:       profile.Plugins,
		PluginConfig:  make([]config.PluginConfig, 0, len(pg)),
	}

	pluginsMap := make(map[string]framework.Plugin)
	for name, factory := range r {
		// initialize only needed plugins.
		if !pg.Has(name) {
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

	// initialize multiPoint plugins to their expanded extension points
	if len(profile.Plugins.MultiPoint.Enabled) > 0 {
		if err := f.expandMultiPointPlugins(profile, pluginsMap); err != nil {
			return nil, err
		}
	}

	if len(f.queueSortPlugins) != 1 {
		return nil, fmt.Errorf("only one queue sort plugin required for profile with scheduler name %q, but got %d", profile.SchedulerName, len(f.queueSortPlugins))
	}
	if len(f.bindPlugins) == 0 {
		return nil, fmt.Errorf("at least one bind plugin is needed for profile with scheduler name %q", profile.SchedulerName)
	}

	if err := getScoreWeights(f, pluginsMap, append(profile.Plugins.Score.Enabled, profile.Plugins.MultiPoint.Enabled...)); err != nil {
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

// waitingPodsMap a thread-safe map used to maintain pods waiting in the permit phase.
type waitingStagesMap struct {
	stages map[string]*waitingStage
	mu     sync.RWMutex
}

// waitingPod represents a pod waiting in the permit phase.
type waitingStage struct {
	stage          *meta.Stage
	pendingPlugins map[string]*time.Timer
	s              chan *framework.Status
	mu             sync.RWMutex
}

// frameworkImpl is the component responsible for initializing and running scheduler
// plugins.
type frameworkImpl struct {
	registry          Registry
	waitingPods       *waitingStagesMap
	scorePluginWeight map[string]int
	queueSortPlugins  []framework.QueueSortPlugin
	filterPlugins     []framework.FilterPlugin
	scorePlugins      []framework.ScorePlugin
	bindPlugins       []framework.BindPlugin
	permitPlugins     []framework.PermitPlugin

	clientSet       interface{}
	config          interface{}
	eventRecorder   interface{}
	informerFactory interface{}

	profileName string

	extenders []framework.Extender
	framework.StageNominator

	parallelizer int
}

func (f *frameworkImpl) RunScorePlugins(ctx context.Context, state *framework.CycleState, stage *meta.Stage, workers []*meta.Worker) (framework.PluginToWorkerScores, *framework.Status) {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) RunFilterPlugins(ctx context.Context, state *framework.CycleState, stage *meta.Stage, info *framework.WorkerInfo) framework.PluginToStatus {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) IterateOverWaitingPods(callback func(framework.WaitingStage)) {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) GetWaitingPod(uid string) framework.WaitingStage {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) RejectWaitingPod(uid string) bool {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) ClientSet() interface{} {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) EventRecorder() interface{} {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) RunFilterPluginsWithNominatedPods(ctx context.Context, state *framework.CycleState, stage *meta.Stage, info *framework.WorkerInfo) *framework.Status {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) Extenders() []framework.Extender {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) Parallelizer() interface{} {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) QueueSortFunc() framework.LessFunc {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) RunPermitPlugins(ctx context.Context, state *framework.CycleState, stage *meta.Stage, workerName string) *framework.Status {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) WaitOnPermit(ctx context.Context, stage *meta.Stage) *framework.Status {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) RunBindPlugins(ctx context.Context, state *framework.CycleState, stage *meta.Stage, workerName string) *framework.Status {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) HasFilterPlugins() bool {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) HasPostFilterPlugins() bool {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) HasScorePlugins() bool {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) ListPlugins() *config.Plugins {
	//TODO implement me
	panic("implement me")
}

func (f *frameworkImpl) ProfileName() string {
	//TODO implement me
	panic("implement me")
}
