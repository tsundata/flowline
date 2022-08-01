package worker

import (
	"context"
	"errors"
	"github.com/tsundata/flowline/pkg/api/client"
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
	config *config.Config

	queue workqueue.RateLimitingInterface

	stageControl  stageControlInterface
	workerControl workerControlInterface

	stageLister       listerv1.StageLister
	stageListerSynced informer.InformerSynced

	// now is a function that returns current time, done to facilitate unit tests
	now func() time.Time
}

func NewController(config *config.Config, stageInformer informerv1.StageInformer, client client.Interface) (*Controller, error) {
	jm := &Controller{
		config:            config,
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "worker"),
		stageControl:      &realStageControl{Client: client},
		workerControl:     &realWorkerControl{Client: client},
		stageLister:       stageInformer.Lister(),
		stageListerSynced: stageInformer.Informer().HasSynced,
		now:               time.Now,
	}

	stageInformer.Informer().AddEventHandler(informer.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *meta.Stage:
				return t.State == meta.StageReady
			case informer.DeletedFinalStateUnknown:
				if w, ok := t.Obj.(*meta.Stage); ok {
					return w.State == meta.StageReady
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

func (jm *Controller) Run(ctx context.Context, workers int) {
	defer parallelizer.HandleCrash()
	defer jm.queue.ShutDown()

	flog.Info("starting worker controller")
	defer flog.Info("shutting down worker controller")

	// register worker
	hostname, _ := os.Hostname()
	_, err := jm.workerControl.Register(ctx, &meta.Worker{
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
	})
	if err != nil {
		flog.Fatalf("error worker register %s %s", jm.config.WorkerID, err)
		return
	}
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
	hostname, _ := os.Hostname()
	_, err := jm.workerControl.Heartbeat(context.Background(), &meta.Worker{
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
	})
	if err != nil {
		flog.Errorf("error worker heartbeat %s %s", jm.config.WorkerID, err)
		return
	}
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
	}

	// Update the job if needed
	if _, err := jm.stageControl.UpdateStage(ctx, stageCopy); err != nil {
		flog.Infof("Unable to update status for stage %s %s %s", stage.GetName(), stage.ResourceVersion, err)
		return false, err
	}

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
	out, err := rt.Run(result.Code, 1000) // fixme
	if err != nil {
		flog.Error(err)
		result.State = meta.StageFailed
		return
	}
	result.Output = out

	// ------------------------------------------------------------------ //
	stage.State = meta.StageSuccess

	return
}
