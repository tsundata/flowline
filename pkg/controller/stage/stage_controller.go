package stage

import (
	"context"
	"errors"
	"github.com/heimdalr/dag"
	"github.com/tsundata/flowline/pkg/api/client"
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
	queue workqueue.RateLimitingInterface

	stageControl stageControlInterface

	stageLister listerv1.StageLister

	stageListerSynced informer.InformerSynced

	// now is a function that returns current time, done to facilitate unit tests
	now func() time.Time
}

func NewController(stageInformer informerv1.StageInformer, client client.Interface) (*Controller, error) {
	jm := &Controller{
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "stage"),

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
				if w, ok := t.Obj.(*meta.Stage); ok {
					return w.State == meta.StageSuccess || w.State == meta.StageFailed
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

func (jm *Controller) enqueueControllerAfter(obj interface{}, t time.Duration) {
	key, err := informer.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		flog.Errorf("couldn't get key for object %s", err)
		return
	}

	jm.queue.AddAfter(key, t)
}

func (jm *Controller) Run(ctx context.Context, workers int) {
	defer parallelizer.HandleCrash()
	defer jm.queue.ShutDown()

	flog.Info("starting stage controller")
	defer flog.Info("shutting down stage controller")

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

	finished, err := jm.depend(ctx, key.(string))
	switch {
	case err != nil:
		flog.Errorf("error syncing stage controller %v, requeuing: %v", key.(string), err)
		jm.queue.AddRateLimited(key)
	case finished:
		jm.queue.Forget(key)
	}
	return true
}

func (jm *Controller) depend(ctx context.Context, jobKey string) (bool, error) {
	name := jobKey
	stage, err := jm.stageLister.Get(name)
	switch {
	case errors.Is(err, informer.ErrNotFound):
		flog.Infof("stage not found, may be it is deleted %s", name)
		return false, nil
	case err != nil:
		return false, err
	}

	stageList, updateStatus, err := jm.dependStage(ctx, stage)
	if err != nil {
		flog.Infof("Error reconciling stage %s %s", stage.GetName(), err)
		if updateStatus {
			if _, err := jm.stageControl.UpdateListStatus(ctx, stageList); err != nil {
				flog.Infof("Unable to update status for job %s %s %s", stage.GetName(), stage.ResourceVersion, err)
				return false, err
			}
		}
		return false, err
	}

	// Update the job if needed
	if updateStatus {
		if _, err := jm.stageControl.UpdateListStatus(ctx, stageList); err != nil {
			flog.Infof("Unable to update status for job %s %s %s", stage.GetName(), stage.ResourceVersion, err)
			return false, err
		}
	}

	return true, nil
}

func (jm *Controller) dependStage(ctx context.Context, upperStage *meta.Stage) (*meta.StageList, bool, error) {
	updateStatus := false

	stageList, err := jm.stageControl.GetStages(ctx)
	if err != nil {
		return nil, false, err
	}
	jobStages := make(map[string]*meta.Stage)
	for i, item := range stageList.Items {
		if item.JobUID == upperStage.JobUID {
			jobStages[item.NodeID] = &stageList.Items[i]
		}
	}

	d := dag.NewDAG()
	for _, item := range jobStages {
		err = d.AddVertexByID(item.NodeID, item.NodeID)
		if err != nil {
			return nil, false, err
		}
	}
	for _, item := range jobStages {
		for _, dependNodeId := range item.DependNodeId {
			err = d.AddEdge(dependNodeId, item.NodeID)
			if err != nil {
				return nil, false, err
			}
		}
	}

	result := &meta.StageList{
		Items: []meta.Stage{},
	}

	switch upperStage.State {
	case meta.StageSuccess:
		children, err := d.GetChildren(upperStage.NodeID)
		if err != nil {
			return nil, false, err
		}

		for nodeId := range children {
			if childrenStage, ok := jobStages[nodeId]; ok && childrenStage != nil {
				allSuccess := true
				var input interface{}
				for _, childrenDependNodeId := range childrenStage.DependNodeId {
					if childrenDependStage, ok := jobStages[childrenDependNodeId]; ok && childrenDependStage != nil {
						if childrenDependStage.State != meta.StageSuccess {
							allSuccess = false
						} else {
							// todo merge input
							input = childrenDependStage.Output
						}
					}
				}
				if allSuccess {
					childrenStage.State = meta.StageReady
					childrenStage.Input = input
					result.Items = append(result.Items, *childrenStage)
				}
			}
		}
	case meta.StageFailed:
		flowCallback := func(d *dag.DAG, id string, parentResults []dag.FlowResult) (interface{}, error) {
			children, err := d.GetChildren(id)
			if err != nil {
				return nil, err
			}
			for nodeId := range children {
				if childrenStage, ok := jobStages[nodeId]; ok && childrenStage != nil {
					if childrenStage.State == meta.StageCreate {
						childrenStage.State = meta.StageFailed
						result.Items = append(result.Items, *childrenStage)
					}
				}
			}
			return nil, nil
		}
		_, err = d.DescendantsFlow(upperStage.NodeID, nil, flowCallback)
	}

	// ------------------------------------------------------------------ //
	if len(result.Items) > 0 {
		updateStatus = true
	}

	return result, updateStatus, nil
}
