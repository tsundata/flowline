package framework

import (
	"context"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/client/events"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/scheduler/framework/config"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"golang.org/x/xerrors"
	"math"
	"strings"
	"sync"
	"time"
)

const (
	// MaxWorkerScore is the maximum score a Score plugin is expected to return.
	MaxWorkerScore int64 = 100

	// MinWorkerScore is the minimum score a Score plugin is expected to return.
	MinWorkerScore int64 = 0

	// MaxTotalScore is the maximum total score.
	MaxTotalScore int64 = math.MaxInt64
)

// WorkerScoreList declares a list of workers and their scores.
type WorkerScoreList []WorkerScore

// WorkerScore is a struct with worker name and score.
type WorkerScore struct {
	UID   string
	Score int64
}

// Plugin is the parent type for all the scheduling framework plugins.
type Plugin interface {
	Name() string
}

// QueueSortPlugin is an interface that must be implemented by "QueueSort" plugins.
// These plugins are used to sort stages in the scheduling queue. Only one queue sort
// plugin may be enabled at a time.
type QueueSortPlugin interface {
	Plugin
	// Less are used to sort stages in the scheduling queue.
	Less(*QueuedStageInfo, *QueuedStageInfo) bool
}

// EnqueueExtensions is an optional interface that plugins can implement to efficiently
// move unschedulable Stages in internal scheduling queues. Plugins
// that fail stage scheduling (e.g., Filter plugins) are expected to implement this interface.
type EnqueueExtensions interface {
	// EventsToRegister returns a series of possible events that may cause a Stage
	// failed by this plugin schedulable.
	// The events will be registered when instantiating the internal scheduling queue,
	// and leveraged to build event handlers dynamically.
	// Note: the returned list needs to be static (not depend on configuration parameters);
	// otherwise it would lead to undefined behavior.
	EventsToRegister() []ClusterEvent
}

// FilterPlugin is an interface for Filter plugins. These plugins are called at the
// filter extension point for filtering out hosts that cannot run a stage.
// This concept used to be called 'predicate' in the original scheduler.
// These plugins should return "Success", "Unschedulable" or "Error" in Status.code.
// However, the scheduler accepts other valid codes as well.
// Anything other than "Success" will lead to exclusion of the given host from
// running the stage.
type FilterPlugin interface {
	Plugin
	// Filter is called by the scheduling framework.
	// All FilterPlugins should return "Success" to declare that
	// the given worker fits the stage. If Filter doesn't return "Success",
	// it will return "Unschedulable", "UnschedulableAndUnresolvable" or "Error".
	// For the worker being evaluated, Filter plugins should look at the passed
	// workerInfo reference for this particular worker's information (e.g., stages
	// considered to be running on the worker) instead of looking it up in the
	// WorkerInfoSnapshot because we don't guarantee that they will be the same.
	// For example, during preemption, we may pass a copy of the original
	// workerInfo object that has some stages removed from it to evaluate the
	// possibility of preempting them to schedule the target stage.
	Filter(ctx context.Context, state *CycleState, stage *meta.Stage, workerInfo *WorkerInfo) *Status
}

// ScoreExtensions is an interface for Score extended functionality.
type ScoreExtensions interface {
	// NormalizeScore is called for all worker scores produced by the same plugin's "Score"
	// method. A successful run of NormalizeScore will update the scores list and return
	// a success status.
	NormalizeScore(ctx context.Context, state *CycleState, p *meta.Stage, scores WorkerScoreList) *Status
}

// ScorePlugin is an interface that must be implemented by "Score" plugins to rank
// workers that passed the filtering phase.
type ScorePlugin interface {
	Plugin
	// Score is called on each filtered worker. It must return success and an integer
	// indicating the rank of the worker. All scoring plugins must return success or
	// the stage will be rejected.
	Score(ctx context.Context, state *CycleState, p *meta.Stage, workerUID string) (int64, *Status)

	// ScoreExtensions returns a ScoreExtensions interface if it implements one, or nil if it does not.
	ScoreExtensions() ScoreExtensions
}

// PermitPlugin is an interface that must be implemented by "Permit" plugins.
// These plugins are called before a stage is bound to a worker.
type PermitPlugin interface {
	Plugin
	// Permit is called before binding a stage (and before prebind plugins). Permit
	// plugins are used to prevent or delay the binding of a Stage. A permit plugin
	// must return success or wait with timeout duration, or the stage will be rejected.
	// The stage will also be rejected if the wait timeout or the stage is rejected while
	// waiting. Note that if the plugin returns "wait", the framework will wait only
	// after running the remaining plugins given that no other plugin rejects the stage.
	Permit(ctx context.Context, state *CycleState, p *meta.Stage, workerUID string) (*Status, time.Duration)
}

// BindPlugin is an interface that must be implemented by "Bind" plugins. Bind
// plugins are used to bind a stage to a Worker.
type BindPlugin interface {
	Plugin
	// Bind plugins will not be called until all pre-bind plugins have completed. Each
	// bind plugin is called in the configured order. A bind plugin may choose whether
	// to handle the given Stage. If a bind plugin chooses to handle a Stage, the
	// remaining bind plugins are skipped. When a bind plugin does not handle a stage,
	// it must return Skip in its Status code. If a bind plugin returns an Error, the
	// stage is rejected and will not be bound.
	Bind(ctx context.Context, state *CycleState, p *meta.Stage, workerUID string) *Status
}

// LessFunc is the function to sort stage info
type LessFunc func(stageInfo1, stageInfo2 *QueuedStageInfo) bool

// Framework manages the set of plugins in use by the scheduling framework.
// Configured plugins are called at specified points in a scheduling context.
type Framework interface {
	Handle
	// QueueSortFunc returns the function to sort stages in scheduling queue
	QueueSortFunc() LessFunc

	// RunPermitPlugins runs the set of configured Permit plugins. If any of these
	// plugins returns a status other than "Success" or "Wait", it does not continue
	// running the remaining plugins and returns an error. Otherwise, if any of the
	// plugins returns "Wait", then this function will create and add waiting stage
	// to a map of currently waiting stages and return status with "Wait" code.
	// Stage will remain waiting stage for the minimum duration returned by the Permit plugins.
	RunPermitPlugins(ctx context.Context, state *CycleState, stage *meta.Stage, workerUID string) *Status

	// WaitOnPermit will block, if the stage is a waiting stage, until the waiting stage is rejected or allowed.
	WaitOnPermit(ctx context.Context, stage *meta.Stage) *Status

	// RunBindPlugins runs the set of configured Bind plugins. A Bind plugin may choose
	// whether to handle the given Stage. If a Bind plugin chooses to skip the
	// binding, it should return code=5("skip") status. Otherwise, it should return "Error"
	// or "Success". If none of the plugins handled binding, RunBindPlugins returns
	// code=5("skip") status.
	RunBindPlugins(ctx context.Context, state *CycleState, stage *meta.Stage, workerUID string) *Status

	// HasFilterPlugins returns true if at least one Filter plugin is defined.
	HasFilterPlugins() bool

	// HasScorePlugins returns true if at least one Score plugin is defined.
	HasScorePlugins() bool

	// ListPlugins returns a map of extension point name to list of configured Plugins.
	ListPlugins() *config.Plugins

	// ProfileName returns the profile name associated to this framework.
	ProfileName() string
}

// Handle provides data and some tools that plugins can use. It is
// passed to the plugin factories at the time of plugin initialization. Plugins
// must store and use this handle to call framework functions.
type Handle interface {
	// StageNominator abstracts operations to maintain nominated Stages.
	StageNominator
	// PluginsRunner abstracts operations to run some plugins.
	PluginsRunner

	// IterateOverWaitingStages acquires a read lock and iterates over the WaitingStages map.
	IterateOverWaitingStages(callback func(WaitingStage))

	// GetWaitingStage returns a waiting stage given its UID.
	GetWaitingStage(uid string) WaitingStage

	// RejectWaitingStage rejects a waiting stage given its UID.
	// The return value indicates if the stage is waiting or not.
	RejectWaitingStage(uid string) bool

	// ClientSet returns a clientSet.
	ClientSet() client.Interface

	// EventRecorder returns an event recorder.
	EventRecorder() events.EventRecorder

	// RunFilterPluginsWithNominatedStages runs the set of configured filter plugins for nominated stage on the given worker.
	RunFilterPluginsWithNominatedStages(ctx context.Context, state *CycleState, stage *meta.Stage, info *WorkerInfo) *Status

	// Extenders returns registered scheduler extenders.
	Extenders() []Extender

	// Parallelizer returns a parallelizer holding parallelism for scheduler.
	Parallelizer() parallelizer.Parallelizer
}

// WaitingStage represents a stage currently waiting in the permit phase.
type WaitingStage interface {
	// GetStage returns a reference to the waiting stage.
	GetStage() *meta.Stage
	// GetPendingPlugins returns a list of pending Permit plugin's name.
	GetPendingPlugins() []string
	// Allow declares the waiting stage is allowed to be scheduled by the plugin named as "pluginName".
	// If this is the last remaining plugin to allow, then a success signal is delivered
	// to unblock the stage.
	Allow(pluginName string)
	// Reject declares the waiting stage unschedulable.
	Reject(pluginName, msg string)
}

// StageNominator abstracts operations to maintain nominated Stages.
type StageNominator interface {
	// AddNominatedStage adds the given stage to the nominator or
	// updates it if it already exists.
	AddNominatedStage(stage *StageInfo, nominatingInfo *NominatingInfo)
	// DeleteNominatedStageIfExists deletes nominatedStage from internal cache. It's a no-op if it doesn't exist.
	DeleteNominatedStageIfExists(stage *meta.Stage)
	// UpdateNominatedStage updates the <oldStage> with <newStage>.
	UpdateNominatedStage(oldStage *meta.Stage, newStageInfo *StageInfo)
	// NominatedStagesForWorker returns nominatedStages on the given worker.
	NominatedStagesForWorker(workerName string) []*StageInfo
}

type NominatingMode int

const (
	ModeNoop NominatingMode = iota
	ModeOverride
)

type NominatingInfo struct {
	NominatedWorkerName string
	NominatingMode      NominatingMode
}

func (ni *NominatingInfo) Mode() NominatingMode {
	if ni == nil {
		return ModeNoop
	}
	return ni.NominatingMode
}

// PluginsRunner abstracts operations to run some plugins.
// This is used by preemption PostFilter plugins when evaluating the feasibility of
// scheduling the stage on workers when certain running stages get evicted.
type PluginsRunner interface {
	// RunScorePlugins runs the set of configured Score plugins. It returns a map that
	// stores for each Score plugin name the corresponding WorkerScoreList(s).
	// It also returns *Status, which is set to non-success if any of the plugins returns
	// a non-success status.
	RunScorePlugins(context.Context, *CycleState, *meta.Stage, []*meta.Worker) (PluginToWorkerScores, *Status)
	// RunFilterPlugins runs the set of configured Filter plugins for stage on
	// the given worker. Note that for the worker being evaluated, the passed workerInfo
	// reference could be different from the one in WorkerInfoSnapshot map (e.g., stages
	// considered to be running on the worker could be different). For example, during
	// preemption, we may pass a copy of the original workerInfo object that has some stages
	// removed from it to evaluate the possibility of preempting them to
	// schedule the target stage.
	RunFilterPlugins(context.Context, *CycleState, *meta.Stage, *WorkerInfo) PluginToStatus
}

// PluginToWorkerScores declares a map from plugin name to its WorkerScoreList.
type PluginToWorkerScores map[string]WorkerScoreList

// WorkerToStatusMap declares map from worker name to its status.
type WorkerToStatusMap map[string]*Status

// PluginToStatus maps plugin name to status. Currently used to identify which Filter plugin
// returned which status.
type PluginToStatus map[string]*Status

// Merge merges the statuses in the map into one. The resulting status code have the following
// precedence: Error, UnschedulableAndUnresolvable, Unschedulable.
func (p PluginToStatus) Merge() *Status {
	if len(p) == 0 {
		return nil
	}

	finalStatus := NewStatus(Success)
	for _, s := range p {
		if s.Code() == Error {
			finalStatus.err = s.AsError()
		}
		if statusPrecedence[s.Code()] > statusPrecedence[finalStatus.code] {
			finalStatus.code = s.Code()
			// Same as code, we keep the most relevant failedPlugin in the returned Status.
			finalStatus.failedPlugin = s.FailedPlugin()
		}

		for _, r := range s.reasons {
			finalStatus.AppendReason(r)
		}
	}

	return finalStatus
}

// Code is the Status code/type which is returned from plugins.
type Code int

// These are predefined codes used in a Status.
const (
	// Success means that plugin ran correctly and found stage schedulable.
	// NOTE: A nil status is also considered as "Success".
	Success Code = iota
	// Error is used for internal plugin errors, unexpected input, etc.
	Error
	// Unschedulable is used when a plugin finds a stage unschedulable. The scheduler might attempt to
	// preempt other stages to get this stage scheduled. Use UnschedulableAndUnresolvable to make the
	// scheduler skip preemption.
	// The accompanying status message should explain why the stage is unschedulable.
	Unschedulable
	// UnschedulableAndUnresolvable is used when a plugin finds a stage unschedulable and
	// preemption would not change anything. Plugins should return Unschedulable if it is possible
	// that the stage can get scheduled with preemption.
	// The accompanying status message should explain why the stage is unschedulable.
	UnschedulableAndUnresolvable
	// Wait is used when a Permit plugin finds a stage scheduling should wait.
	Wait
	// Skip is used when a Bind plugin chooses to skip binding.
	Skip
)

// statusPrecedence defines a map from status to its precedence, larger value means higher precedent.
var statusPrecedence = map[Code]int{
	Error:                        3,
	UnschedulableAndUnresolvable: 2,
	Unschedulable:                1,
	// Any other statuses we know today, `Skip` or `Wait`, will take precedence over `Success`.
	Success: -1,
}

// Status indicates the result of running a plugin. It consists of a code, a
// message, (optionally) an error, and a plugin name it fails by.
// When the status code is not Success, the reasons should explain why.
// And, when code is Success, all the other fields should be empty.
// NOTE: A nil Status is also considered as Success.
type Status struct {
	code    Code
	reasons []string
	err     error
	// failedPlugin is an optional field that records the plugin name a Stage failed by.
	// It's set by the framework when code is Error, Unschedulable or UnschedulableAndUnresolvable.
	failedPlugin string
}

// Code returns code of the Status.
func (s *Status) Code() Code {
	if s == nil {
		return Success
	}
	return s.code
}

// Message returns a concatenated message on reasons of the Status.
func (s *Status) Message() string {
	if s == nil {
		return ""
	}
	return strings.Join(s.reasons, ", ")
}

// SetFailedPlugin sets the given plugin name to s.failedPlugin.
func (s *Status) SetFailedPlugin(plugin string) {
	s.failedPlugin = plugin
}

// WithFailedPlugin sets the given plugin name to s.failedPlugin,
// and returns the given status object.
func (s *Status) WithFailedPlugin(plugin string) *Status {
	s.SetFailedPlugin(plugin)
	return s
}

// FailedPlugin returns the failed plugin name.
func (s *Status) FailedPlugin() string {
	return s.failedPlugin
}

// Reasons returns reasons of the Status.
func (s *Status) Reasons() []string {
	return s.reasons
}

// AppendReason appends given reason to the Status.
func (s *Status) AppendReason(reason string) {
	s.reasons = append(s.reasons, reason)
}

// IsSuccess returns true if and only if "Status" is nil or Code is "Success".
func (s *Status) IsSuccess() bool {
	return s.Code() == Success
}

// IsWait returns true if and only if "Status" is non-nil and its Code is "Wait".
func (s *Status) IsWait() bool {
	return s.Code() == Wait
}

// IsSkip returns true if and only if "Status" is non-nil and its Code is "Skip".
func (s *Status) IsSkip() bool {
	return s.Code() == Skip
}

// IsUnschedulable returns true if "Status" is Unschedulable (Unschedulable or UnschedulableAndUnresolvable).
func (s *Status) IsUnschedulable() bool {
	code := s.Code()
	return code == Unschedulable || code == UnschedulableAndUnresolvable
}

// AsError returns nil if the status is a success or a wait; otherwise returns an "error" object
// with a concatenated message on reasons of the Status.
func (s *Status) AsError() error {
	if s.IsSuccess() || s.IsWait() || s.IsSkip() {
		return nil
	}
	if s.err != nil {
		return s.err
	}
	return xerrors.New(s.Message())
}

// Equal checks equality of two statuses. This is useful for testing with
// cmp.Equal.
func (s *Status) Equal(x *Status) bool {
	if s == nil || x == nil {
		return s.IsSuccess() && x.IsSuccess()
	}
	if s.code != x.code {
		return false
	}
	if s.code == Error {
		return cmp.Equal(s.err, x.err, cmpopts.EquateErrors())
	}
	return cmp.Equal(s.reasons, x.reasons)
}

// NewStatus makes a Status out of the given arguments and returns its pointer.
func NewStatus(code Code, reasons ...string) *Status {
	s := &Status{
		code:    code,
		reasons: reasons,
	}
	if code == Error {
		s.err = xerrors.New(s.Message())
	}
	return s
}

// AsStatus wraps an error in a Status.
func AsStatus(err error) *Status {
	return &Status{
		code:    Error,
		reasons: []string{err.Error()},
		err:     err,
	}
}

// StageToActivateKey is a reserved state key for stashing stages.
// If the stashed stages are present in unschedulableStages or backoffQ，they will be
// activated (i.e., moved to activeQ) in two phases:
// - end of a scheduling cycle if it succeeds (will be cleared from `StagesToActivate` if activated)
// - end of a binding cycle if it succeeds
var StageToActivateKey StateKey = "apps/stages-to-activate"

// StageToActivate stores stages to be activated.
type StageToActivate struct {
	sync.Mutex
	// Map is keyed with namespaced stage name, and valued with the stage.
	Map map[string]*meta.Stage
}

// Clone just returns the same state.
func (s *StageToActivate) Clone() StateData {
	return s
}

// NewStageToActivate instantiates a StageToActivate object.
func NewStageToActivate() *StageToActivate {
	return &StageToActivate{Map: make(map[string]*meta.Stage)}
}
