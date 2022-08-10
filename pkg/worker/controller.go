package worker

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
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"github.com/tsundata/flowline/pkg/util/workqueue"
	"github.com/tsundata/flowline/pkg/worker/config"
	"github.com/tsundata/flowline/pkg/worker/sandbox"
	"os"
	"time"
)

type Controller struct {
	config   *config.Config
	recorder record.EventRecorder

	queue workqueue.RateLimitingInterface

	stageControl  stageControlInterface
	workerControl workerControlInterface

	stageLister       listerv1.StageLister
	stageListerSynced informer.InformerSynced

	// now is a function that returns current time, done to facilitate unit tests
	now func() time.Time
}

func NewController(config *config.Config, stageInformer informerv1.StageInformer, client client.Interface) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging("")
	eventBroadcaster.StartRecordingToSink(&events.EventSinkImpl{Interface: client.EventsV1()})

	jm := &Controller{
		config:   config,
		queue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "worker"),
		recorder: eventBroadcaster.NewRecorder(v1.Scheme, meta.EventSource{Component: "worker"}),

		stageControl:  &realStageControl{Client: client},
		workerControl: &realWorkerControl{Client: client},

		stageLister:       stageInformer.Lister(),
		stageListerSynced: stageInformer.Informer().HasSynced,

		now: time.Now,
	}

	stageInformer.Informer().AddEventHandler(informer.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *meta.Stage:
				return t.State == meta.StageBind && t.WorkerUID == config.WorkerID
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
	_, okOld := old.(*meta.Stage)
	_, okNew := cur.(*meta.Stage)
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

func (jm *Controller) workerMeta() *meta.Worker {
	hostname, _ := os.Hostname()
	return &meta.Worker{
		TypeMeta: meta.TypeMeta{
			Kind:       "worker",
			APIVersion: constant.Version,
		},
		ObjectMeta: meta.ObjectMeta{
			Name: "worker-" + jm.config.WorkerID,
			UID:  jm.config.WorkerID,
		},
		State:    meta.WorkerReady,
		Hostname: hostname,
		Runtimes: jm.config.Runtime,
	}
}

func (jm *Controller) Run(ctx context.Context, workers int) {
	defer parallelizer.HandleCrash()
	defer jm.queue.ShutDown()

	flog.Info("starting worker controller")
	defer flog.Info("shutting down worker controller")

	// register worker
	worker := jm.workerMeta()
	_, err := jm.workerControl.Register(ctx, worker)
	if err != nil {
		jm.recorder.Eventf(worker, meta.EventTypeWarning, "FailedRegister", "worker %s register failed", jm.config.WorkerID)
		flog.Fatalf("error worker register %s %s", jm.config.WorkerID, err)
		return
	}
	jm.recorder.Eventf(worker, meta.EventTypeNormal, "SuccessfulRegister", "worker %s register successful", jm.config.WorkerID)

	// worker heartbeat
	go parallelizer.Until(jm.heartbeat, time.Minute, ctx.Done())

	if !informer.WaitForNamedCacheSync("worker", ctx.Done(), jm.stageListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go parallelizer.UntilWithContext(ctx, jm.worker, time.Second)
	}

	<-ctx.Done()
}

func (jm *Controller) heartbeat() {
	worker := jm.workerMeta()
	_, err := jm.workerControl.Heartbeat(context.Background(), worker)
	if err != nil {
		jm.recorder.Eventf(worker, meta.EventTypeWarning, "FailedHeartbeat", "worker %s heartbeat failed", jm.config.WorkerID)
		flog.Errorf("error worker heartbeat %s %s", jm.config.WorkerID, err)
		return
	}
	jm.recorder.Eventf(worker, meta.EventTypeNormal, "SuccessfulHeartbeat", "worker %s heartbeat successful", jm.config.WorkerID)
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

	finished, err := jm.execute(ctx, key.(string))
	switch {
	case err != nil:
		flog.Errorf("error syncing worker controller %v, requeuing: %v", key.(string), err)
		jm.queue.AddRateLimited(key)
	case finished:
		jm.queue.Forget(key)
	}
	return true
}

func (jm *Controller) execute(ctx context.Context, stageKey string) (bool, error) {
	name := stageKey
	stage, err := jm.stageLister.Get(name)
	switch {
	case errors.Is(err, informer.ErrNotFound):
		flog.Infof("stage not found, may be it is deleted %s", name)
		return false, nil
	case err != nil:
		return false, err
	}

	stageCopy, err := jm.executeCode(ctx, stage)
	if err != nil {
		flog.Infof("Error reconciling stage %s %s", stage.GetName(), err)
		if _, err := jm.stageControl.UpdateStage(ctx, stageCopy); err != nil {
			flog.Infof("Unable to update status for stage %s %s %s", stage.GetName(), stage.ResourceVersion, err)
			return false, err
		}
		jm.recorder.Eventf(stageCopy, meta.EventTypeWarning, "FailedExecute", "Stage %s execute failed %v on worker %s", stageCopy.UID, err, jm.config.WorkerID)
	}

	// Update the job if needed
	if _, err := jm.stageControl.UpdateStage(ctx, stageCopy); err != nil {
		flog.Infof("Unable to update status for stage %s %s %s", stage.GetName(), stage.ResourceVersion, err)
		return false, err
	}

	jm.recorder.Eventf(stageCopy, meta.EventTypeNormal, "SuccessfulExecute", "Stage %s execute success on worker %s", stageCopy.UID, jm.config.WorkerID)

	return true, nil
}

func (jm *Controller) executeCode(_ context.Context, stage *meta.Stage) (result *meta.Stage, err error) {
	defer func() {
		if r := recover(); r != nil {
			flog.Errorf("stage run error %v", r)
			result.State = meta.StageFailed
		}
	}()
	result = stage

	rt := sandbox.Factory(sandbox.RuntimeType(result.Runtime))
	out, err := rt.Run(result.Code, stage.Input)
	if err != nil {
		flog.Error(err)
		result.State = meta.StageFailed
		return
	}

	// ------------------------------------------------------------------ //
	result.State = meta.StageSuccess
	result.Output = out

	return
}
