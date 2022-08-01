package scheduler

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/informer"
	"github.com/tsundata/flowline/pkg/informer/informers"
	"github.com/tsundata/flowline/pkg/scheduler/framework"
	"github.com/tsundata/flowline/pkg/scheduler/queue"
	"github.com/tsundata/flowline/pkg/util/flog"
)

func addAllEventHandlers(sched *Scheduler, informerFactory informers.SharedInformerFactory, _ map[framework.GVK]framework.ActionType) {
	// scheduled stage cache
	informerFactory.Core().V1().Stages().Informer().AddEventHandler(
		informer.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *meta.Stage:
					return assignedStage(t)
				case informer.DeletedFinalStateUnknown:
					if _, ok := t.Obj.(*meta.Stage); ok {
						return true
					}
					flog.Errorf("unable to convert object %T to *meta.stage in %T", obj, sched)
					return false
				default:
					flog.Errorf("unable to handle object in %T: %T", sched, obj)
					return false
				}
			},
			Handler: informer.ResourceEventHandlerFuncs{
				AddFunc:    sched.addStageToCache,
				UpdateFunc: sched.updateStageInCache,
				DeleteFunc: sched.deleteStageFromCache,
			},
		},
	)
	// unscheduled stage queue
	informerFactory.Core().V1().Stages().Informer().AddEventHandler(
		informer.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *meta.Stage:
					return !assignedStage(t)
				case informer.DeletedFinalStateUnknown:
					if _, ok := t.Obj.(*meta.Stage); ok {
						return true
					}
					flog.Errorf("unable to convert object %T to *meta.stage in %T", obj, sched)
					return false
				default:
					flog.Errorf("unable to handle object in %T: %T", sched, obj)
					return false
				}
			},
			Handler: informer.ResourceEventHandlerFuncs{
				AddFunc:    sched.addStageToSchedulingQueue,
				UpdateFunc: sched.updateStageInSchedulingQueue,
				DeleteFunc: sched.deleteStageFromSchedulingQueue,
			},
		},
	)

	informerFactory.Core().V1().Workers().Informer().AddEventHandler(
		informer.ResourceEventHandlerFuncs{
			AddFunc:    sched.addWorkerToCache,
			UpdateFunc: sched.updateWorkerToCache,
			DeleteFunc: sched.deleteWorkerToCache,
		},
	)
}

func assignedStage(stage *meta.Stage) bool {
	return stage.State != meta.StageCreate || stage.WorkerUID != ""
}

func (sched *Scheduler) addStageToSchedulingQueue(obj interface{}) {
	stage := obj.(*meta.Stage)
	flog.Infof("Add event for unscheduled stage %v", stage.UID)
	if err := sched.SchedulingQueue.Add(stage); err != nil {
		flog.Errorf("unable to queue %T: %v", obj, err)
	}
}

func (sched *Scheduler) updateStageInSchedulingQueue(oldObj, newObj interface{}) {
	oldStage, newStage := oldObj.(*meta.Stage), newObj.(*meta.Stage)
	if oldStage.ResourceVersion == newStage.ResourceVersion {
		return
	}

	isAssumed, err := sched.Cache.IsAssumedStage(newStage)
	if err != nil {
		flog.Errorf("failed to check whether stage %s/%s is assumed: %v", newStage.Name, newStage.UID, err)
	}
	if isAssumed {
		return
	}

	if err := sched.SchedulingQueue.Update(oldStage, newStage); err != nil {
		flog.Errorf("unable to update %T: %v", newObj, err)
	}
}

func (sched *Scheduler) deleteStageFromSchedulingQueue(obj interface{}) {
	var stage *meta.Stage
	switch t := obj.(type) {
	case *meta.Stage:
		stage = t
	case informer.DeletedFinalStateUnknown:
		var ok bool
		stage, ok = t.Obj.(*meta.Stage)
		if !ok {
			flog.Errorf("Cannot convert to *meta.Stage %T", t.Obj)
			return
		}
	default:
		flog.Errorf("Cannot convert to *meta.Stage %T", t)
		return
	}
	flog.Infof("delete event for unscheduled stage %T", stage)

	if err := sched.SchedulingQueue.Delete(stage); err != nil {
		flog.Errorf("unable to dequeue %T", stage)
	}
	fwk, err := sched.frameworkForStage(stage)
	if err != nil {
		flog.Errorf("Unable to get profile %T", stage)
		return
	}

	if fwk.RejectWaitingStage(stage.UID) {
		sched.SchedulingQueue.MoveAllToActiveOrBackoffQueue(queue.AssignedStageDelete, nil)
	}
}

func (sched *Scheduler) addStageToCache(obj interface{}) {
	stage, ok := obj.(*meta.Stage)
	if !ok {
		flog.Errorf("cannot convert to *meta.Stage %T", obj)
		return
	}
	flog.Infof("add event for scheduled stage %T", stage)

	if err := sched.Cache.AddStage(stage); err != nil {
		flog.Errorf("scheduler cache addStage failed %T", stage)
	}

	sched.SchedulingQueue.AssignedStageAdded(stage)
}

func (sched *Scheduler) updateStageInCache(oldObj, newObj interface{}) {
	oldStage, ok := oldObj.(*meta.Stage)
	if !ok {
		flog.Errorf("cannot convert to *meta.Stage %T", oldObj)
		return
	}
	newStage, ok := newObj.(*meta.Stage)
	if !ok {
		flog.Errorf("cannot convert to *meta.Stage %T", newObj)
		return
	}

	flog.Infof("update event for scheduled stage %+v --> %+v", oldStage.UID, newStage.UID)

	if err := sched.Cache.UpdateStage(oldStage, newStage); err != nil {
		flog.Errorf("scheduler cache updateStage failed %+v --> %+v", oldStage.UID, newStage.UID)
	}

	sched.SchedulingQueue.AssignedStageUpdated(newStage)
}

func (sched *Scheduler) deleteStageFromCache(obj interface{}) {
	var stage *meta.Stage
	switch t := obj.(type) {
	case *meta.Stage:
		stage = t
	case informer.DeletedFinalStateUnknown:
		var ok bool
		stage, ok = t.Obj.(*meta.Stage)
		if !ok {
			flog.Errorf("Cannot convert to *meta.Stage %T", t.Obj)
			return
		}
	default:
		flog.Errorf("Cannot convert to *meta.Stage %T", t)
		return
	}
	flog.Infof("delete event for scheduled stage %T", stage)

	if err := sched.Cache.RemoveStage(stage); err != nil {
		flog.Errorf("scheduler cache removeStage failed %T", stage)
	}

	sched.SchedulingQueue.MoveAllToActiveOrBackoffQueue(queue.AssignedStageDelete, nil)
}

func (sched *Scheduler) addWorkerToCache(obj interface{}) {
	worker, ok := obj.(*meta.Worker)
	if !ok {
		flog.Errorf("cannot convert to *meta.worker %T", obj)
		return
	}
	flog.Infof("add event for worker %+v", worker.UID)
	workerInfo := sched.Cache.AddWorker(worker)
	sched.SchedulingQueue.MoveAllToActiveOrBackoffQueue(queue.WorkerAdd, preCheckForWorker(workerInfo))
}

func (sched *Scheduler) updateWorkerToCache(oldObj, newObj interface{}) {
	oldWorker, ok := oldObj.(*meta.Worker)
	if !ok {
		flog.Errorf("cannot convert to *meta.Worker %T", oldObj)
		return
	}
	newWorker, ok := newObj.(*meta.Worker)
	if !ok {
		flog.Errorf("cannot convert to *meta.Worker %T", newObj)
		return
	}

	flog.Infof("update event for worker %+v --> %+v", oldWorker.UID, newWorker.UID)

	workerInfo := sched.Cache.UpdateWorker(oldWorker, newWorker)
	if event := workerSchedulingPropertiesChange(newWorker, oldWorker); event != nil {
		sched.SchedulingQueue.MoveAllToActiveOrBackoffQueue(*event, preCheckForWorker(workerInfo))
	}
}

func (sched *Scheduler) deleteWorkerToCache(obj interface{}) {
	var worker *meta.Worker
	switch t := obj.(type) {
	case *meta.Worker:
		worker = t
	case informer.DeletedFinalStateUnknown:
		var ok bool
		worker, ok = t.Obj.(*meta.Worker)
		if !ok {
			flog.Errorf("Cannot convert to *meta.Worker %T", t.Obj)
			return
		}
	default:
		flog.Errorf("Cannot convert to *meta.Worker %T", t)
		return
	}

	flog.Infof("delete event for worker %T", worker)
	if err := sched.Cache.RemoveWorker(worker); err != nil {
		flog.Errorf("scheduler cache RemoveWorker failed")
	}
}

func workerSchedulingPropertiesChange(newWorker, oldWorker *meta.Worker) *framework.ClusterEvent {
	if workerStateChanged(newWorker, oldWorker) {
		return &queue.WorkerStateChange
	}
	return nil
}

func workerStateChanged(newWorker, oldWorker *meta.Worker) bool {
	return !(newWorker.State == oldWorker.State)
}

func preCheckForWorker(_ *framework.WorkerInfo) queue.PreEnqueueCheck {
	return func(pod *meta.Stage) bool {
		return true
	}
}
