package scheduler

import (
	"context"
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/scheduler/queue"
	"github.com/tsundata/flowline/pkg/util/flog"
	"math/rand"
	"sync"
)

var clearNominatedWorker = &framework.NominatingInfo{NominatingMode: framework.ModeOverride, NominatedWorkerName: ""}

func (sched *Scheduler) scheduleOne(ctx context.Context) {
	stageInfo := sched.NextStage()
	// stage could be nil when schedulerQueue is closed
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
		nominatingInfo = clearNominatedWorker
		sched.handleSchedulingFailure(ctx, fwk, stageInfo, err, "Unschedulable", nominatingInfo)
		return
	}

	assumedStageInfo := stageInfo // todo deep copy
	assumedStage := assumedStageInfo.Stage
	err = sched.assume(assumedStage, scheduleResult.SuggestedHost)
	if err != nil {
		sched.handleSchedulingFailure(ctx, fwk, stageInfo, err, "SchedulerError", clearNominatedWorker)
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
		sched.handleSchedulingFailure(ctx, fwk, assumedStageInfo, runPermitStatus.AsError(), reason, clearNominatedWorker)
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
				defer sched.SchedulingQueue.MoveAllToActiveOrBackoffQueue(queue.AssignedStageDelete, func(stage *meta.Stage) bool {
					return assumedStage.UID != stage.UID
				})
			}
			sched.handleSchedulingFailure(ctx, fwk, assumedStageInfo, waitOnPermitStatus.AsError(), reason, clearNominatedWorker)
			return
		}

		err := sched.bind(bindingCycleCtx, fwk, assumedStage, scheduleResult.SuggestedHost, state)
		if err != nil {
			if forgetErr := sched.Cache.ForgetStage(assumedStage); forgetErr != nil {
				flog.Error(forgetErr)
			} else {
				sched.SchedulingQueue.MoveAllToActiveOrBackoffQueue(queue.AssignedStageDelete, nil)
			}
		}

		if len(stagesToActivate.Map) != 0 {
			sched.SchedulingQueue.Activate(stagesToActivate.Map)
		}
	}()
	flog.Info("scheduleOne end")
}

func (sched *Scheduler) assume(assumed *meta.Stage, uid string) error {
	assumed.WorkerUID = uid

	if err := sched.Cache.AssumeStage(assumed); err != nil {
		flog.Error(err)
		return err
	}

	if sched.SchedulingQueue != nil {
		sched.SchedulingQueue.DeleteNominatedStageIfExists(assumed)
	}

	return nil
}

// bind binds a stage to a given worker defined in a binding object.
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

func (sched *Scheduler) finishBinding(fwk framework.Framework, assumed *meta.Stage, targetWorker string, err error) {
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
		"Type":    "StageScheduled",
		"Status":  "ConditionFalse",
		"Reason":  reason,
		"Message": err.Error(),
	}, nominatingInfo); err != nil {
		flog.Error(err)
	}
}

func (sched *Scheduler) frameworkForStage(stage *meta.Stage) (framework.Framework, error) {
	stage.SchedulerName = constant.DefaultSchedulerName // fixme
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
	// todo  	_, err = cs.CoreV1().Stages(old.Namespace).Patch(ctx, old.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}, "status")

	return nil
}

func prioritizeWorkers(
	ctx context.Context,
	extenders []framework.Extender,
	fwk framework.Framework,
	state *framework.CycleState,
	stage *meta.Stage,
	workers []*meta.Worker,
) (framework.WorkerScoreList, error) {
	// If no priority configs are provided, then all workers will have a score of one.
	// This is required to generate the priority list in the required format
	if len(extenders) == 0 && !fwk.HasScorePlugins() {
		result := make(framework.WorkerScoreList, 0, len(workers))
		for i := range workers {
			result = append(result, framework.WorkerScore{
				UID:   workers[i].UID,
				Score: 1,
			})
		}
		return result, nil
	}

	// Run the Score plugins.
	scoresMap, scoreStatus := fwk.RunScorePlugins(ctx, state, stage, workers)
	if !scoreStatus.IsSuccess() {
		return nil, scoreStatus.AsError()
	}

	// Additional details logged at level 10 if enabled.
	for plugin, workerScoreList := range scoresMap {
		for _, workerScore := range workerScoreList {
			flog.Infof("Plugin scored worker for stage %s %s, %s %v %v", stage.Name, stage.UID, plugin, workerScore.UID, workerScore.Score)
		}
	}

	// Summarize all scores.
	result := make(framework.WorkerScoreList, 0, len(workers))

	for i := range workers {
		result = append(result, framework.WorkerScore{UID: workers[i].UID, Score: 0})
		for j := range scoresMap {
			result[i].Score += scoresMap[j][i].Score
		}
	}

	if len(extenders) != 0 && workers != nil {
		var mu sync.Mutex
		var wg sync.WaitGroup
		combinedScores := make(map[string]int64, len(workers))
		for i := range extenders {
			if !extenders[i].IsInterested(stage) {
				continue
			}
			wg.Add(1)
			go func(extIndex int) {
				prioritizedList, weight, err := extenders[extIndex].Prioritize(stage, workers)
				if err != nil {
					// Prioritization errors from extender can be ignored, let k8s/other extenders determine the priorities
					flog.Infof("Failed to run extender's priority function. No score given by this extender. %s %v %s", err, stage, extenders[extIndex].Name())
					return
				}
				mu.Lock()
				for i := range *prioritizedList {
					host, score := (*prioritizedList)[i].Host, (*prioritizedList)[i].Score
					flog.Infof("Extender scored worker for stage, %v %s %v %v", stage, extenders[extIndex].Name(), host, score)
					combinedScores[host] += score * weight
				}
				mu.Unlock()
			}(i)
		}
		// wait for all go routines to finish
		wg.Wait()
		for i := range result {
			// MaxExtenderPriority may diverge from the max priority used in the scheduler and defined by MaxWorkerScore,
			// therefore we need to scale the score returned by extenders to the score range used by the scheduler.
			result[i].Score += combinedScores[result[i].UID] * (framework.MaxWorkerScore / MaxExtenderPriority)
		}
	}

	for i := range result {
		flog.Infof("Calculated worker's final score for stage %s %s, %s %v", stage.Name, stage.UID, result[i].UID, result[i].Score)
	}

	return result, nil
}

const MaxExtenderPriority = 10

// selectHost takes a prioritized list of workers and then picks one
// in a reservoir sampling manner from the workers that had the highest score.
func selectHost(workerScoreList framework.WorkerScoreList) (string, error) {
	if len(workerScoreList) == 0 {
		return "", fmt.Errorf("empty priorityList")
	}
	maxScore := workerScoreList[0].Score
	selected := workerScoreList[0].UID
	cntOfMaxScore := 1
	for _, ns := range workerScoreList[1:] {
		if ns.Score > maxScore {
			maxScore = ns.Score
			selected = ns.UID
			cntOfMaxScore = 1
		} else if ns.Score == maxScore {
			cntOfMaxScore++
			if rand.Intn(cntOfMaxScore) == 0 {
				// Replace the candidate with probability of 1/cntOfMaxScore
				selected = ns.UID
			}
		}
	}
	return selected, nil
}
