package crontrigger

import (
	"context"
	"errors"
	"fmt"
	"github.com/robfig/cron/v3"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/informer"
	informerv1 "github.com/tsundata/flowline/pkg/informer/informers/core/v1"
	listerv1 "github.com/tsundata/flowline/pkg/informer/listers/core/v1"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"github.com/tsundata/flowline/pkg/util/workqueue"
	"sort"
	"time"
)

type Controller struct {
	queue workqueue.RateLimitingInterface

	jobControl      jobControlInterface
	workflowControl cjControlInterface

	jobLister      listerv1.JobLister
	workflowLister listerv1.WorkflowLister

	jobListerSynced      informer.InformerSynced
	workflowListerSynced informer.InformerSynced

	// now is a function that returns current time, done to facilitate unit tests
	now func() time.Time
}

func NewController(jobInformer informerv1.JobInformer, workflowInformer informerv1.WorkflowInformer, client client.Interface) (*Controller, error) {
	jm := &Controller{
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "crontrigger"),

		jobControl:      &realJobControl{Client: client},
		workflowControl: &realCJControl{Client: client},

		jobLister:      jobInformer.Lister(),
		workflowLister: workflowInformer.Lister(),

		jobListerSynced:      jobInformer.Informer().HasSynced,
		workflowListerSynced: workflowInformer.Informer().HasSynced,
		now:                  time.Now,
	}

	jobInformer.Informer().AddEventHandler(informer.ResourceEventHandlerFuncs{
		AddFunc:    jm.addJob,
		UpdateFunc: jm.updateJob,
		DeleteFunc: jm.deleteJob,
	})

	workflowInformer.Informer().AddEventHandler(informer.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *meta.Workflow:
				return t.Trigger == meta.TriggerCron
			case informer.DeletedFinalStateUnknown:
				if _, ok := t.Obj.(*meta.Workflow); ok {
					return true
				}
				flog.Errorf("unable to convert object %T to *meta.Workflow", obj)
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
			UpdateFunc: jm.updateWorkflow,
			DeleteFunc: func(obj interface{}) {
				jm.enqueueController(obj)
			},
		},
	})

	return jm, nil
}

func (jm *Controller) addJob(obj interface{}) {
	job := obj.(*meta.Job)
	if job.DeletionTimestamp != nil {
		jm.deleteJob(job)
		return
	}

	if job.WorkflowUID != "" {
		workflow, err := jm.workflowLister.Get(job.WorkflowUID)
		if err != nil {
			return
		}
		if workflow == nil {
			return
		}
		jm.enqueueController(workflow)
	}
}

func (jm *Controller) updateJob(old, cur interface{}) {
	curJob := cur.(*meta.Job)
	oldObj := old.(*meta.Job)
	if curJob.ResourceVersion == oldObj.ResourceVersion {
		return
	}

	if curJob.WorkflowUID != "" {
		workflow, err := jm.workflowLister.Get(curJob.WorkflowUID)
		if err != nil {
			return
		}
		if workflow == nil {
			return
		}
		jm.enqueueController(workflow)
		return
	}
}

func (jm *Controller) deleteJob(obj interface{}) {
	job, ok := obj.(*meta.Job)

	if !ok {
		tombstone, ok := obj.(informer.DeletedFinalStateUnknown)
		if !ok {
			flog.Errorf("couldn't get object from tombstone %v", obj)
			return
		}
		job, ok = tombstone.Obj.(*meta.Job)
		if !ok {
			flog.Errorf("tombstone contained object that is not a job %v", obj)
			return
		}
	}

	workflow, err := jm.workflowLister.Get(job.WorkflowUID)
	if err != nil {
		return
	}
	if workflow == nil {
		return
	}
	jm.enqueueController(workflow)
}

func (jm *Controller) updateWorkflow(old, cur interface{}) {
	oldCJ, okOld := old.(*meta.Workflow)
	newCJ, okNew := cur.(*meta.Workflow)

	if !okOld || !okNew {
		return
	}
	// if the change in schedule results in next requeue having to be sooner than it already was,
	// it will be handled here by the queue. If the next requeue is further than previous schedule,
	// the sync loop will essentially be a no-op for the already queued key with old schedule.
	if oldCJ.TriggerParam != newCJ.TriggerParam {
		sched, err := cron.ParseStandard(newCJ.TriggerParam)
		if err != nil {
			flog.Infof("unparseable schedule form crontrigger %s %s", newCJ.UID, newCJ.TriggerParam)
			return
		}
		now := jm.now()
		t := nextScheduledTimeDuration(*newCJ, sched, now)

		jm.enqueueControllerAfter(cur, *t)
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

	cur, _ := obj.(*meta.Workflow)
	if !cur.Active {
		flog.Infof("crontrigger forget workflow key %s", key)
		jm.queue.Forget(key)
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
	cronJob, err := jm.workflowLister.Get(name)
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

	cronJobCopy, requeueAfter, updateStatus, err := jm.syncCronJob(ctx, cronJob, jobsToBeReconciled)
	if err != nil {
		flog.Infof("Error reconciling crontrigger %s %s", cronJob.GetName(), err)
		if updateStatus {
			if _, err := jm.workflowControl.UpdateStatus(ctx, cronJobCopy); err != nil {
				flog.Infof("Unable to update status for crontrigger %s %s %s", cronJob.GetName(), cronJob.ResourceVersion, err)
				return nil, err
			}
		}
		return nil, err
	}

	if jm.cleanupFinishedJobs(ctx, cronJobCopy, jobsToBeReconciled) {
		updateStatus = true
	}

	// Update the CronJob if needed
	if updateStatus {
		if _, err := jm.workflowControl.UpdateStatus(ctx, cronJobCopy); err != nil {
			flog.Infof("Unable to update status for crontrigger %s %s %s", cronJob.GetName(), cronJob.ResourceVersion, err)
			return nil, err
		}
	}

	if requeueAfter != nil {
		flog.Infof("Re-queuing crontrigger %s %s", cronJob.GetName(), requeueAfter)
		return requeueAfter, nil
	}

	return nil, nil
}

func (jm *Controller) getJobsToBeReconciled(cj *meta.Workflow) ([]*meta.Job, error) {
	jobList, err := jm.jobLister.List(nil)
	if err != nil {
		return nil, err
	}

	var jobsToBeReconciled []*meta.Job

	for i, job := range jobList {
		if cj.UID == job.WorkflowUID {
			jobsToBeReconciled = append(jobsToBeReconciled, jobList[i])
		}
	}

	return jobsToBeReconciled, nil
}

// cleanupFinishedJobs cleanups finished jobs created by a CronJob
// It returns a bool to indicate an update to api-server is needed
func (jm *Controller) cleanupFinishedJobs(_ context.Context, cj *meta.Workflow, js []*meta.Job) bool {
	// If neither limits are active, there is no need to do anything.
	if cj.FailedJobsHistoryLimit == nil && cj.SuccessfulJobsHistoryLimit == nil {
		return false
	}

	updateStatus := false
	var failedJobs []*meta.Job
	var successfulJobs []*meta.Job

	for _, job := range js {
		isFinished, finishedStatus := jm.getFinishedStatus(job)
		if isFinished && finishedStatus == meta.JobSuccess {
			successfulJobs = append(successfulJobs, job)
		} else if isFinished && finishedStatus == meta.JobFailed {
			failedJobs = append(failedJobs, job)
		}
	}

	if cj.SuccessfulJobsHistoryLimit != nil &&
		jm.removeOldestJobs(cj,
			successfulJobs,
			*cj.SuccessfulJobsHistoryLimit) {
		updateStatus = true
	}

	if cj.FailedJobsHistoryLimit != nil &&
		jm.removeOldestJobs(cj,
			failedJobs,
			*cj.FailedJobsHistoryLimit) {
		updateStatus = true
	}

	return updateStatus
}

// removeOldestJobs removes the oldest jobs from a list of jobs
func (jm *Controller) removeOldestJobs(cj *meta.Workflow, js []*meta.Job, maxJobs int32) bool {
	updateStatus := false
	numToDelete := len(js) - int(maxJobs)
	if numToDelete <= 0 {
		return false
	}

	flog.Infof("Cleaning up jobs from CronJob list %d %d %s", numToDelete, len(js), cj.GetName())

	sort.Sort(byJobStartTimeStar(js))
	for i := 0; i < numToDelete; i++ {
		flog.Infof("Removing job from CronJob list %s %s", js[i].Name, cj.GetName())
		if deleteJob(cj, js[i], jm.jobControl) {
			updateStatus = true
		}
	}
	return updateStatus
}

// deleteJob reaps a job, deleting the job, the workflow and the reference in the active list
func deleteJob(cj *meta.Workflow, job *meta.Job, jc jobControlInterface) bool {
	nameForLog := cj.Name

	// delete the job itself...
	if err := jc.DeleteJob(job.Name); err != nil {
		flog.Errorf("Error deleting job %s from %s: %v", job.Name, nameForLog, err)
		return false
	}
	// ... and its reference from active list
	deleteFromActiveList(cj, job.ObjectMeta.UID)
	flog.Infof("Deleted job %v", job.Name)

	return true
}

// byJobStartTimeStar sorts a list of jobs by start timestamp, using their names as a tiebreaker.
type byJobStartTimeStar []*meta.Job

func (o byJobStartTimeStar) Len() int      { return len(o) }
func (o byJobStartTimeStar) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

func (o byJobStartTimeStar) Less(i, j int) bool {
	if o[i].StartTime == nil && o[j].StartTime != nil {
		return false
	}
	if o[i].StartTime != nil && o[j].StartTime == nil {
		return true
	}
	if o[i].StartTime.Equal(*o[j].StartTime) {
		return o[i].Name < o[j].Name
	}
	return o[i].StartTime.Before(*o[j].StartTime)
}

func (jm *Controller) getFinishedStatus(j *meta.Job) (bool, meta.JobState) {
	if j.State == meta.JobSuccess || j.State == meta.JobFailed {
		return true, j.State
	}
	return false, ""
}

// IsJobFinished returns whether a job has completed successfully or failed.
func IsJobFinished(j *meta.Job) bool {
	isFinished, _ := getFinishedStatus(j)
	return isFinished
}

func getFinishedStatus(j *meta.Job) (bool, meta.JobState) {
	if j.State == meta.JobSuccess || j.State == meta.JobFailed {
		return true, j.State
	}
	return false, ""
}

// syncCronJob reconciles a CronJob with a list of any Jobs that it created.
// All known jobs created by "cronJob" should be included in "jobs".
// The current time is passed in to facilitate testing.
// It returns a copy of the CronJob that is to be used by other functions
// that mutates the object
// It also returns a bool to indicate an update to api-server is needed
func (jm *Controller) syncCronJob(
	ctx context.Context,
	cronJob *meta.Workflow,
	jobs []*meta.Job) (*meta.Workflow, *time.Duration, bool, error) {

	now := jm.now()
	updateStatus := false

	childrenJobs := make(map[string]bool)
	for _, j := range jobs {
		childrenJobs[j.ObjectMeta.UID] = true
		found := inActiveList(*cronJob, j.ObjectMeta.UID)
		if !found && !IsJobFinished(j) {
			cjCopy, err := jm.workflowControl.GetWorkflow(ctx, cronJob.UID)
			if err != nil {
				return nil, nil, updateStatus, err
			}
			if inActiveList(*cjCopy, j.ObjectMeta.UID) {
				cronJob = cjCopy
				continue
			}
			flog.Infof("Saw a job that the controller did not create or forgot: %s", j.Name)
			// We found an unfinished job that has us as the parent, but it is not in our Active list.
			// This could happen if we crashed right after creating the Job and before updating the status,
			// or if our jobs list is newer than our cj status after a relist, or if someone intentionally created
			// a job that they wanted us to adopt.
		} else if found && IsJobFinished(j) {
			_, status := getFinishedStatus(j)
			deleteFromActiveList(cronJob, j.ObjectMeta.UID)
			flog.Infof("Saw completed job: %s, status: %v", j.Name, status)
			updateStatus = true
		} else if IsJobFinished(j) {
			// a job does not have to be in active list, as long as it is finished, we will process the timestamp
			if cronJob.LastSuccessfulTimestamp == nil {
				cronJob.LastSuccessfulTimestamp = j.CompletionTimestamp
				updateStatus = true
			}
			if j.CompletionTimestamp != nil && j.CompletionTimestamp.After(*cronJob.LastSuccessfulTimestamp) {
				cronJob.LastSuccessfulTimestamp = j.CompletionTimestamp
				updateStatus = true
			}
		}
	}

	if cronJob.DeletionTimestamp != nil {
		// The CronJob is being deleted.
		// Don't do anything other than updating status.
		return cronJob, nil, updateStatus, nil
	}

	sched, err := cron.ParseStandard(cronJob.TriggerParam)
	if err != nil {
		// this is likely a user error in defining the spec value
		// we should log the error and not reconcile this crontrigger until an update to spec
		flog.Errorf("unparseable schedule: %q : %s", cronJob.TriggerParam, err)
		return cronJob, nil, updateStatus, nil
	}

	scheduledTime, err := getNextScheduleTime(*cronJob, now, sched)
	if err != nil {
		// this is likely a user error in defining the spec value
		// we should log the error and not reconcile this crontrigger until an update to spec
		flog.Errorf("invalid schedule: %s : %s", cronJob.TriggerParam, err)
		return cronJob, nil, updateStatus, nil
	}
	if scheduledTime == nil {
		// no unmet start time, return cj,
		// The only time this should happen is if queue is filled after restart.
		// Otherwise, the queue is always supposed to trigger sync function at the time of
		// the scheduled time, that will give atleast 1 unmet time schedule
		flog.Infof("No unmet start times %s", cronJob.GetUID())
		t := nextScheduledTimeDuration(*cronJob, sched, now)
		return cronJob, t, updateStatus, nil
	}

	tooLate := false
	if cronJob.StartingDeadlineSeconds != nil {
		tooLate = scheduledTime.Add(time.Second * time.Duration(*cronJob.StartingDeadlineSeconds)).Before(now)
	}
	if tooLate {
		flog.Infof("Missed scheduled time to start a job: %s", scheduledTime.UTC().Format(time.RFC1123Z))

		// Since we don't set LastScheduleTime when not scheduling, we are going to keep noticing
		// the miss every cycle.  In order to avoid sending multiple events, and to avoid processing
		// the cj again and again, we could set a Status.LastMissedTime when we notice a miss.
		// Then, when we call getRecentUnmetScheduleTimes, we can take max(creationTimestamp,
		// Status.LastScheduleTime, Status.LastMissedTime), and then so we won't generate
		// and event the next time we process it, and also so the user looking at the status
		// can see easily that there was a missed execution.
		t := nextScheduledTimeDuration(*cronJob, sched, now)
		return cronJob, t, updateStatus, nil
	}
	if cronJob.LastTriggerTimestamp != nil && cronJob.LastTriggerTimestamp.Equal(*scheduledTime) {
		flog.Infof("Not starting job because the scheduled time is already processed %s %s", cronJob.GetUID(), scheduledTime.UTC().Format(time.RFC1123Z))
		t := nextScheduledTimeDuration(*cronJob, sched, now)
		return cronJob, t, updateStatus, nil
	}

	jobReq, err := getJobFromTemplate(cronJob, *scheduledTime)
	if err != nil {
		flog.Errorf("unable to make job from template %s", cronJob.UID)
		return cronJob, nil, updateStatus, err
	}
	jobResp, err := jm.jobControl.CreateJob(jobReq)
	if err != nil {
		flog.Errorf("failed create cron job %s", jobReq.Name)
		return cronJob, nil, updateStatus, err
	}

	flog.Infof("Created Job %s %s", jobResp.GetUID(), cronJob.GetUID())

	// ------------------------------------------------------------------ //

	// If this process restarts at this point (after posting a job, but
	// before updating the status), then we might try to start the job on
	// the next time.  Actually, if we re-list the SJs and Jobs on the next
	// iteration of syncAll, we might not see our own status update, and
	// then post one again.  So, we need to use the job name as a lock to
	// prevent us from making the job twice (name the job with hash of its
	// scheduled time).
	cronJob.LastTriggerTimestamp = scheduledTime
	updateStatus = true

	t := nextScheduledTimeDuration(*cronJob, sched, now)
	return cronJob, t, updateStatus, nil
}

// getJobFromTemplate2 makes a Job from a CronJob. It converts the unix time into minutes from
// epoch time and concatenates that to the job name, because the crontrigger_controller v2 has the lowest
// granularity of 1 minute for scheduling job.
func getJobFromTemplate(cj *meta.Workflow, scheduledTime time.Time) (*meta.Job, error) {
	// We want job names for a given nominal start time to have a deterministic name to avoid the same job being created twice
	name := getJobName(cj, scheduledTime)

	job := &meta.Job{
		TypeMeta: meta.TypeMeta{
			Kind:       "job",
			APIVersion: constant.Version,
		},
		ObjectMeta: meta.ObjectMeta{
			Name:              name,
			CreationTimestamp: &scheduledTime,
		},
		// todo
		State:       meta.JobCreate,
		WorkflowUID: cj.UID,
	}

	return job, nil
}

func getJobName(cj *meta.Workflow, scheduledTime time.Time) string {
	return fmt.Sprintf("%s-%d", cj.Name, getTimeHashInMinutes(scheduledTime))
}

// getTimeHash returns Unix Epoch Time in minutes
func getTimeHashInMinutes(scheduledTime time.Time) int64 {
	return scheduledTime.Unix() / 60
}

func deleteFromActiveList(cj *meta.Workflow, _ string) {
	if cj == nil {
		return
	}
}

func inActiveList(cj meta.Workflow, uid string) bool {
	if cj.Active && cj.UID == uid {
		return true
	}
	return false
}

// getNextScheduleTime gets the time of next schedule after last scheduled and before now
//  it returns nil if no unmet schedule times.
// If there are too many (>100) unstarted times, it will raise a warning and still return
// the list of missed times.
func getNextScheduleTime(cj meta.Workflow, now time.Time, schedule cron.Schedule) (*time.Time, error) {
	var (
		earliestTime time.Time
	)
	if cj.LastTriggerTimestamp != nil {
		earliestTime = *cj.LastTriggerTimestamp
	} else {
		// If none found, then this is either a recently created cronJob,
		// or the active/completed info was somehow lost (contract for status
		// in kubernetes says it may need to be recreated), or that we have
		// started a job, but have not noticed it yet (distributed systems can
		// have arbitrary delays).  In any case, use the creation time of the
		// CronJob as last known start time.
		earliestTime = *cj.ObjectMeta.CreationTimestamp
	}
	if cj.StartingDeadlineSeconds != nil {
		// Controller is not going to schedule anything below this point
		schedulingDeadline := now.Add(-time.Second * time.Duration(*cj.StartingDeadlineSeconds))

		if schedulingDeadline.After(earliestTime) {
			earliestTime = schedulingDeadline
		}
	}
	if earliestTime.After(now) {
		return nil, nil
	}

	t, numberOfMissedSchedules, err := getMostRecentScheduleTime(earliestTime, now, schedule)

	if numberOfMissedSchedules > 100 {
		// An object might miss several starts. For example, if
		// controller gets wedged on friday at 5:01pm when everyone has
		// gone home, and someone comes in on tuesday AM and discovers
		// the problem and restarts the controller, then all the hourly
		// jobs, more than 80 of them for one hourly cronJob, should
		// all start running with no further intervention (if the cronJob
		// allows concurrency and late starts).
		//
		// However, if there is a bug somewhere, or incorrect clock
		// on controller's server or apiservers (for setting creationTimestamp)
		// then there could be so many missed start times (it could be off
		// by decades or more), that it would eat up all the CPU and memory
		// of this controller. In that case, we want to not try to list
		// all the missed start times.
		//
		// I've somewhat arbitrarily picked 100, as more than 80,
		// but less than "lots".
		flog.Infof("too many missed times %s %d", cj.GetName(), numberOfMissedSchedules)
	}
	return t, err
}

// getMostRecentScheduleTime returns the latest schedule time between earliestTime and the count of number of
// schedules in between them
func getMostRecentScheduleTime(earliestTime time.Time, now time.Time, schedule cron.Schedule) (*time.Time, int64, error) {
	t1 := schedule.Next(earliestTime)
	t2 := schedule.Next(t1)

	if now.Before(t1) {
		return nil, 0, nil
	}
	if now.Before(t2) {
		return &t1, 1, nil
	}

	// It is possible for cron.ParseStandard("59 23 31 2 *") to return an invalid schedule
	// seconds - 59, minute - 23, hour - 31 (?!)  dom - 2, and dow is optional, clearly 31 is invalid
	// In this case the timeBetweenTwoSchedules will be 0, and we error out the invalid schedule
	timeBetweenTwoSchedules := int64(t2.Sub(t1).Round(time.Second).Seconds())
	if timeBetweenTwoSchedules < 1 {
		return nil, 0, fmt.Errorf("time difference between two schedules less than 1 second")
	}
	timeElapsed := int64(now.Sub(t1).Seconds())
	numberOfMissedSchedules := (timeElapsed / timeBetweenTwoSchedules) + 1
	t := time.Unix(t1.Unix()+((numberOfMissedSchedules-1)*timeBetweenTwoSchedules), 0).UTC()
	return &t, numberOfMissedSchedules, nil
}

var nextScheduleDelta = 100 * time.Millisecond

// nextScheduledTimeDuration returns the time duration to requeue based on
// the schedule and last schedule time. It adds a 100ms padding to the next requeue to account
// for Network Time Protocol(NTP) time skews. If the time drifts are adjusted which in most
// realistic cases would be around 100s, scheduled cron will still be executed without missing
// the schedule.
func nextScheduledTimeDuration(cj meta.Workflow, sched cron.Schedule, now time.Time) *time.Duration {
	earliestTime := *cj.ObjectMeta.CreationTimestamp
	if cj.LastTriggerTimestamp != nil {
		earliestTime = *cj.LastTriggerTimestamp
	}
	mostRecentTime, _, err := getMostRecentScheduleTime(earliestTime, now, sched)
	if err != nil {
		// we still have to requeue at some point, so aim for the next scheduling slot from now
		mostRecentTime = &now
	} else if mostRecentTime == nil {
		// no missed schedules since earliestTime
		mostRecentTime = &earliestTime
	}

	t := sched.Next(*mostRecentTime).Add(nextScheduleDelta).Sub(now)

	return &t
}
