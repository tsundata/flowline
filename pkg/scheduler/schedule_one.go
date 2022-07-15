package scheduler

import (
	"context"
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/scheduler/queue"
	"github.com/tsundata/flowline/pkg/util/flog"
)

var clearNominatedNode = &framework.NominatingInfo{NominatingMode: framework.ModeOverride, NominatedNodeName: ""}

func (sched *Scheduler) scheduleOne(ctx context.Context) {
	stageInfo := sched.NextStage()
	// pod could be nil when schedulerQueue is closed
	if stageInfo == nil || stageInfo.Stage == nil {
		return
	}
	stage := stageInfo.Stage
	fwk, err := sched.frameworkForStage(stage)
	if err != nil {
		flog.Error(err)
		return
	}
	if sched.skipStageSchedule(fwk, stage) {
		return
	}

	flog.Infof("Attempting to schedule stage %s", stage.UID)

	state := framework.NewCycleState()
	stagesToActivate := framework.NewStageToActivate()
	state.Write(framework.StageToActivateKey, stagesToActivate)

	schedulingCycleCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	scheduleResult, err := sched.ScheduleStage(schedulingCycleCtx, fwk, state, stage)
	if err != nil {
		var nominatingInfo *framework.NominatingInfo
		nominatingInfo = clearNominatedNode
		sched.handleSchedulingFailure(ctx, fwk, stageInfo, err, "Unschedulable", nominatingInfo)
		return
	}

	assumedStageInfo := stageInfo // todo deep copy
	assumedStage := assumedStageInfo.Stage
	err = sched.assume(assumedStage, scheduleResult.SuggestedHost)
	if err != nil {
		sched.handleSchedulingFailure(ctx, fwk, stageInfo, err, "SchedulerError", clearNominatedNode)
		return
	}

	// Run "permit" plugins.
	runPermitStatus := fwk.RunPermitPlugins(schedulingCycleCtx, state, assumedStage, scheduleResult.SuggestedHost)
	if !runPermitStatus.IsWait() && !runPermitStatus.IsSuccess() {
		var reason string
		if runPermitStatus.IsUnschedulable() {
			reason = "Unschedulable"
		} else {
			reason = "SchedulerError"
		}
		if forgetErr := sched.Cache.ForgetStage(assumedStage); forgetErr != nil {
			flog.Error(forgetErr)
		}
		sched.handleSchedulingFailure(ctx, fwk, assumedStageInfo, runPermitStatus.AsError(), reason, clearNominatedNode)
		return
	}

	if len(stagesToActivate.Map) != 0 {
		sched.SchedulingQueue.Activate(stagesToActivate.Map)
		stagesToActivate.Map = make(map[string]*meta.Stage)
	}

	go func() {
		bindingCycleCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		waitOnPermitStatus := fwk.WaitOnPermit(bindingCycleCtx, assumedStage)
		if !waitOnPermitStatus.IsSuccess() {
			var reason string
			if waitOnPermitStatus.IsUnschedulable() {
				reason = "Unschedulable"
			} else {
				reason = "SchedulerError"
			}

			if forgetErr := sched.Cache.ForgetStage(assumedStage); forgetErr != nil {
				flog.Error(forgetErr)
			} else {
				defer sched.SchedulingQueue.MoveAllToActiveOrBackoffQueue(queue.AssignedPodDelete, func(stage *meta.Stage) bool {
					return assumedStage.UID != stage.UID
				})
			}
			sched.handleSchedulingFailure(ctx, fwk, assumedStageInfo, waitOnPermitStatus.AsError(), reason, clearNominatedNode)
			return
		}

		err := sched.bind(bindingCycleCtx, fwk, assumedStage, scheduleResult.SuggestedHost, state)
		if err != nil {
			if forgetErr := sched.Cache.ForgetStage(assumedStage); forgetErr != nil {
				flog.Error(forgetErr)
			} else {
				sched.SchedulingQueue.MoveAllToActiveOrBackoffQueue(queue.AssignedPodDelete, nil)
			}
		}

		if len(stagesToActivate.Map) != 0 {
			sched.SchedulingQueue.Activate(stagesToActivate.Map)
		}
	}()
}

func (sched *Scheduler) assume(assumed *meta.Stage, host string) error {
	assumed.WorkerHost = host

	if err := sched.Cache.AssumeStage(assumed); err != nil {
		flog.Error(err)
		return err
	}

	if sched.SchedulingQueue != nil {
		sched.SchedulingQueue.DeleteNominatedStageIfExists(assumed)
	}

	return nil
}

// bind binds a pod to a given node defined in a binding object.
// The precedence for binding is: (1) extenders and (2) framework plugins.
// We expect this to run asynchronously, so we handle binding metrics internally.
func (sched *Scheduler) bind(ctx context.Context, fwk framework.Framework, assumed *meta.Stage, targetWorker string, state *framework.CycleState) (err error) {
	defer func() {
		sched.finishBinding(fwk, assumed, targetWorker, err)
	}()

	bound, err := sched.extendersBinding(assumed, targetWorker)
	if bound {
		return err
	}
	bindStatus := fwk.RunBindPlugins(ctx, state, assumed, targetWorker)
	if bindStatus.IsSuccess() {
		return nil
	}
	if bindStatus.Code() == framework.Error {
		return bindStatus.AsError()
	}
	return fmt.Errorf("bind status: %v, %v", bindStatus.Code(), bindStatus.Message())
}

func (sched *Scheduler) extendersBinding(stage *meta.Stage, worker string) (bool, error) {
	for _, extender := range sched.Extenders {
		if !extender.IsBinder() || !extender.IsInterested(stage) {
			continue
		}
		return true, extender.Bind(&meta.Binding{
			ObjectMeta: meta.ObjectMeta{Name: stage.Name, UID: stage.UID},
			Target:     &meta.Worker{ObjectMeta: meta.ObjectMeta{UID: worker}},
		})
	}
	return false, nil
}

func (sched *Scheduler) finishBinding(fwk framework.Framework, assumed *meta.Stage, targetNode string, err error) {
	if finErr := sched.Cache.FinishBinding(assumed); finErr != nil {
		flog.Error(finErr)
	}
	if err != nil {
		flog.Error(err)
	}
	// todo fwk.EventRecorder "Scheduled"
}

func (sched *Scheduler) handleSchedulingFailure(ctx context.Context, fwk framework.Framework, stageInfo *framework.QueuedStageInfo, err error, reason string, nominatingInfo *framework.NominatingInfo) {
	sched.Error(stageInfo, err)

	if sched.SchedulingQueue != nil {
		sched.SchedulingQueue.AddNominatedStage(stageInfo.StageInfo, nominatingInfo)
	}

	stage := stageInfo.Stage
	// todo fwk.EventRecorder "FailedScheduling"
	if err := updateStage(ctx, sched.client, stage, map[string]interface{}{
		"Type":    "PodScheduled",
		"Status":  "ConditionFalse",
		"Reason":  reason,
		"Message": err.Error(),
	}, nominatingInfo); err != nil {
		flog.Error(err)
	}
}

func (sched *Scheduler) frameworkForStage(stage *meta.Stage) (framework.Framework, error) {
	fwk, ok := sched.Profiles[stage.SchedulerName]
	if !ok {
		return nil, fmt.Errorf("profile not found for scheduler name %q", stage.SchedulerName)
	}
	return fwk, nil
}

func (sched *Scheduler) skipStageSchedule(fwk framework.Framework, stage *meta.Stage) bool {
	if stage.DeletionTimestamp != nil {
		flog.Infof("skip schedule deleting stage %s", stage.UID)
		return true
	}
	isAssumed := false // todo
	return isAssumed
}

func updateStage(ctx context.Context, client interface{}, stage *meta.Stage, condition interface{}, nominatingInfo *framework.NominatingInfo) error {
	// todo  	_, err = cs.CoreV1().Pods(old.Namespace).Patch(ctx, old.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}, "status")

	return nil
}
