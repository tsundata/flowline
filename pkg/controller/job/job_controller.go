package job

import (
	"context"
	"errors"
	"github.com/tsundata/flowline/pkg/api/client"
	v1 "github.com/tsundata/flowline/pkg/api/client/core/v1"
	"github.com/tsundata/flowline/pkg/api/client/events"
	"github.com/tsundata/flowline/pkg/api/client/record"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/informer"
	informerv1 "github.com/tsundata/flowline/pkg/informer/informers/core/v1"
	listerv1 "github.com/tsundata/flowline/pkg/informer/listers/core/v1"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"github.com/tsundata/flowline/pkg/util/workqueue"
	"time"
)

type Controller struct {
	queue    workqueue.RateLimitingInterface
	recorder record.EventRecorder

	jobControl   jobControlInterface
	stageControl stageControlInterface

	stageLister listerv1.StageLister

	stageListerSynced informer.InformerSynced

	// now is a function that returns current time, done to facilitate unit tests
	now func() time.Time
}

func NewController(stageInformer informerv1.StageInformer, client client.Interface) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging("")
	eventBroadcaster.StartRecordingToSink(&events.EventSinkImpl{Interface: client.EventsV1()})

	jm := &Controller{
		queue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "job"),
		recorder: eventBroadcaster.NewRecorder(v1.Scheme, meta.EventSource{Component: "job-controller"}),

		jobControl:   &realJobControl{Client: client},
		stageControl: &realStageControl{Client: client},

		stageLister:       stageInformer.Lister(),
		stageListerSynced: stageInformer.Informer().HasSynced,

		now: time.Now,
	}

	stageInformer.Informer().AddEventHandler(informer.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *meta.Stage:
				return t.State == meta.StageSuccess || t.State == meta.StageFailed
			case informer.DeletedFinalStateUnknown:
				if _, ok := t.Obj.(*meta.Stage); ok {
					return true
				}
				flog.Errorf("unable to convert object %T to *meta.Job", obj)
				return false
			default:
				flog.Errorf("unable to handle object in %T", obj)
				return false
			}
		},
		Handler: informer.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				jm.enqueueController(obj)
			},
			UpdateFunc: jm.updateStage,
			DeleteFunc: func(obj interface{}) {
				jm.enqueueController(obj)
			},
		},
	})

	return jm, nil
}

func (jm *Controller) updateStage(old, cur interface{}) {
	_, okOld := old.(*meta.Job)
	_, okNew := cur.(*meta.Job)
	if !okOld || !okNew {
		return
	}

	jm.enqueueController(cur)
}

func (jm *Controller) enqueueController(obj interface{}) {
	key, err := informer.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		flog.Errorf("couldn't get key for object %s", err)
		return
	}

	jm.queue.Add(key)
}

func (jm *Controller) Run(ctx context.Context, workers int) {
	defer parallelizer.HandleCrash()
	defer jm.queue.ShutDown()

	flog.Info("starting job controller")
	defer flog.Info("shutting down job controller")

	if !informer.WaitForNamedCacheSync("stage", ctx.Done(), jm.stageListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go parallelizer.UntilWithContext(ctx, jm.worker, time.Second)
	}

	<-ctx.Done()
}

func (jm *Controller) worker(ctx context.Context) {
	for jm.processNextWorkItem(ctx) {
	}
}

func (jm *Controller) processNextWorkItem(ctx context.Context) bool {
	key, quit := jm.queue.Get()
	if quit {
		return false
	}
	defer jm.queue.Done(key)

	finished, err := jm.complete(ctx, key.(string))
	switch {
	case err != nil:
		flog.Errorf("error syncing job controller %v, requeuing: %v", key.(string), err)
		jm.queue.AddRateLimited(key)
	case finished:
		jm.queue.Forget(key)
	}
	return true
}

func (jm *Controller) complete(ctx context.Context, stageKey string) (bool, error) {
	name := stageKey
	stage, err := jm.stageLister.Get(name)
	switch {
	case errors.Is(err, informer.ErrNotFound):
		flog.Infof("stage not found, may be it is deleted %s", name)
		return false, nil
	case err != nil:
		return false, err
	}

	job, updateStatus, err := jm.completeJob(ctx, stage)
	if err != nil {
		flog.Infof("Error reconciling stage %s %s", stage.GetName(), err)
		return false, err
	}

	// Update the job if needed
	if updateStatus {
		if _, err := jm.jobControl.UpdateStatus(ctx, job); err != nil {
			flog.Infof("Unable to update status for job %s %s %s", stage.GetName(), stage.ResourceVersion, err)
			return false, err
		}
		if job.State == meta.JobSuccess {
			jm.recorder.Eventf(job, meta.EventTypeNormal, "UpdateSuccessState", "Job %v update success state", job.UID)
		}
		if job.State == meta.JobFailed {
			jm.recorder.Eventf(job, meta.EventTypeNormal, "UpdateFailedState", "Job %v update failed state", job.UID)
		}
	}

	return true, nil
}

func (jm *Controller) completeJob(ctx context.Context, stage *meta.Stage) (*meta.Job, bool, error) {
	updateStatus := false

	stageList, err := jm.stageControl.GetStages(ctx)
	if err != nil {
		return nil, false, err
	}
	var checkStage []*meta.Stage
	for i, item := range stageList.Items {
		if item.JobUID == stage.JobUID {
			checkStage = append(checkStage, &stageList.Items[i])
		}
	}

	stageCount := len(checkStage)
	successCount := 0
	failedCount := 0
	for _, item := range checkStage {
		if item.State == meta.StageSuccess {
			successCount++
		}
		if item.State == meta.StageFailed {
			failedCount++
		}
	}

	if stageCount > 0 {
		job, err := jm.jobControl.GetJob(stage.JobUID)
		if err != nil {
			return nil, false, err
		}
		now := time.Now()
		if stageCount == successCount {
			job.State = meta.JobSuccess
			job.CompletionTimestamp = &now
			updateStatus = true
		}
		if failedCount > 0 {
			job.State = meta.JobFailed
			job.CompletionTimestamp = &now
			updateStatus = true
		}
		return job, updateStatus, nil
	}

	return nil, false, nil
}
