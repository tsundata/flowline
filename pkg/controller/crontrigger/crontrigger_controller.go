package crontrigger

import (
	"context"
	"errors"
	"github.com/tsundata/flowline/pkg/api/client"
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

	workflowLister listerv1.WorkflowLister
	jobLister      listerv1.JobLister

	jobListerSynced informer.InformerSynced

	// now is a function that returns current time, done to facilitate unit tests
	now func() time.Time
}

func NewController(workflowInformer informerv1.WorkflowInformer, jobInformer informerv1.JobInformer, client client.Interface) (*Controller, error) {

	jm := &Controller{
		queue:           nil, // todo
		workflowLister:  workflowInformer.Lister(),
		jobLister:       jobInformer.Lister(),
		jobListerSynced: jobInformer.Informer().HasSynced,
		now:             time.Now,
	}

	workflowInformer.Informer().AddEventHandler(informer.ResourceEventHandlerFuncs{
		AddFunc:    jm.addWorkflow,
		UpdateFunc: jm.updateWorkflow,
		DeleteFunc: jm.deleteWorkflow,
	})

	jobInformer.Informer().AddEventHandler(informer.ResourceEventHandlerFuncs{
		AddFunc:    nil,
		UpdateFunc: nil,
		DeleteFunc: nil,
	})

	return jm, nil
}

func (jm *Controller) Run(ctx context.Context, workers int) {
	defer parallelizer.HandleCrash()
	defer jm.queue.ShutDown()

	flog.Info("starting crontrigger controller")
	defer flog.Info("shutting down crontrigger controller")

	if !informer.WaitForNamedCacheSync("crontrigger", ctx.Done(), jm.jobListerSynced) {
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

	requeueAfter, err := jm.sync(ctx, key.(string))
	switch {
	case err != nil:
		flog.Errorf("error syncing crontrigger controller %v, requeuing: %v", key.(string), err)
		jm.queue.AddRateLimited(key)
	case requeueAfter != nil:
		jm.queue.Forget(key)
		jm.queue.AddAfter(key, *requeueAfter)
	}
	return true
}

func (jm *Controller) sync(ctx context.Context, cronJobKey string) (*time.Duration, error) {
	name := cronJobKey
	cronJob, err := jm.jobLister.Get(name)
	switch {
	case errors.Is(err, informer.ErrNotFound):
		flog.Infof("cron job not found, may be it is deleted %s", name)
		return nil, nil
	case err != nil:
		return nil, err
	}

	jobsToBeReconciled, err := jm.getJobsToBeReconciled(cronJob)
	if err != nil {
		return nil, err
	}

}
