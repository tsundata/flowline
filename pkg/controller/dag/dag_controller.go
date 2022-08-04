package dag

import (
	"context"
	"errors"
	"fmt"
	dagLib "github.com/heimdalr/dag"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/informer"
	informerv1 "github.com/tsundata/flowline/pkg/informer/informers/core/v1"
	listerv1 "github.com/tsundata/flowline/pkg/informer/listers/core/v1"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"github.com/tsundata/flowline/pkg/util/sets"
	"github.com/tsundata/flowline/pkg/util/workqueue"
	"time"
)

type Controller struct {
	queue workqueue.RateLimitingInterface

	jobControl   jobControlInterface
	stageControl stageControlInterface

	codeControl       codeControlInterface
	variableControl   variableControlInterface
	connectionControl connectionControlInterface

	jobLister listerv1.JobLister

	jobListerSynced informer.InformerSynced

	// now is a function that returns current time, done to facilitate unit tests
	now func() time.Time
}

func NewController(jobInformer informerv1.JobInformer, client client.Interface) (*Controller, error) {
	jm := &Controller{
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "dag"),

		jobControl:   &realJobControl{Client: client},
		stageControl: &realStageControl{Client: client},

		codeControl:       &realCodeControl{Client: client},
		variableControl:   &realVariableControl{Client: client},
		connectionControl: &realConnectionControl{Client: client},

		jobLister:       jobInformer.Lister(),
		jobListerSynced: jobInformer.Informer().HasSynced,

		now: time.Now,
	}

	jobInformer.Informer().AddEventHandler(informer.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *meta.Job:
				return t.State == meta.JobCreate
			case informer.DeletedFinalStateUnknown:
				if _, ok := t.Obj.(*meta.Job); ok {
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
			UpdateFunc: jm.updateJob,
			DeleteFunc: func(obj interface{}) {
				jm.enqueueController(obj)
			},
		},
	})

	return jm, nil
}

func (jm *Controller) updateJob(old, cur interface{}) {
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

	flog.Info("starting dag controller")
	defer flog.Info("shutting down dag controller")

	if !informer.WaitForNamedCacheSync("dag", ctx.Done(), jm.jobListerSynced) {
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

	finished, err := jm.split(ctx, key.(string))
	switch {
	case err != nil:
		flog.Errorf("error syncing dag controller %v, requeuing: %v", key.(string), err)
		jm.queue.AddRateLimited(key)
	case finished:
		jm.queue.Forget(key)
	}
	return true
}

func (jm *Controller) split(ctx context.Context, jobKey string) (bool, error) {
	name := jobKey
	job, err := jm.jobLister.Get(name)
	switch {
	case errors.Is(err, informer.ErrNotFound):
		flog.Infof("job not found, may be it is deleted %s", name)
		return false, nil
	case err != nil:
		return false, err
	}

	dag, err := jm.jobControl.GetDag(ctx, job.WorkflowUID)
	if err != nil {
		flog.Infof("dag not found, may be it is deleted %s", name)
		return false, nil
	}

	jobCopy, updateStatus, err := jm.splitDag(ctx, job, dag)
	if err != nil {
		flog.Infof("Error reconciling dag %s %s", job.GetName(), err)
		if updateStatus {
			if _, err := jm.jobControl.UpdateStatus(ctx, jobCopy); err != nil {
				flog.Infof("Unable to update status for job %s %s %s", job.GetName(), job.ResourceVersion, err)
				return false, err
			}
		}
		return false, err
	}

	// Update the job if needed
	if updateStatus {
		if _, err := jm.jobControl.UpdateStatus(ctx, jobCopy); err != nil {
			flog.Infof("Unable to update status for job %s %s %s", job.GetName(), job.ResourceVersion, err)
			return false, err
		}
	}

	return true, nil
}

func (jm *Controller) splitDag(ctx context.Context, job *meta.Job, dag *meta.Dag) (*meta.Job, bool, error) {
	stages, err := dagSort(dag)
	if err != nil {
		return nil, false, err
	}

	codeUID := sets.NewString()
	variableUID := sets.NewString()
	connectionUID := sets.NewString()
	for _, stage := range stages {
		codeUID.Insert(stage.Code)
		variableUID.Insert(stage.Variables...)
		connectionUID.Insert(stage.Connections...)
	}
	codes := make(map[string]*meta.Code)
	if codeUID.Len() > 0 {
		codeList, err := jm.codeControl.GetCodes(ctx)
		if err != nil {
			return nil, false, err
		}
		if codeList == nil {
			return nil, false, errors.New("error code")
		}
		for i, item := range codeList.Items {
			if codeUID.Has(item.UID) {
				codes[item.UID] = &codeList.Items[i]
			}
		}
	}
	variables := make(map[string]*meta.Variable)
	if len(variableUID) > 0 {
		variableList, err := jm.variableControl.GetVariables(ctx)
		if err != nil {
			return nil, false, err
		}
		if variableList == nil {
			return nil, false, errors.New("error variable")
		}
		for i, item := range variableList.Items {
			if variableUID.Has(item.UID) {
				variables[item.UID] = &variableList.Items[i]
			}
		}
	}
	connections := make(map[string]*meta.Connection)
	if len(connectionUID) > 0 {
		connectionList, err := jm.connectionControl.GetConnections(ctx)
		if err != nil {
			return nil, false, err
		}
		if connectionList == nil {
			return nil, false, errors.New("error connection")
		}
		for i, item := range connectionList.Items {
			if connectionUID.Has(item.UID) {
				connections[item.UID] = &connectionList.Items[i]
			}
		}
	}

	updateStatus := false
	for i, item := range stages {
		stageReq, err := getStageFromTemplate(i, job, item, codes, variables, connections)
		if err != nil {
			flog.Errorf("unable to make job from template %s", job.Name)
			return job, false, err
		}
		stageResp, err := jm.stageControl.CreateStage(stageReq)
		if err != nil {
			flog.Errorf("failed create stage %s", stageReq.Name)
			return job, false, err
		}

		flog.Infof("Created Stage %s %s", stageResp.GetName(), job.GetName())
	}

	// ------------------------------------------------------------------ //

	job.State = meta.JobStage
	updateStatus = true

	return job, updateStatus, nil
}

type nodeId string

func (n nodeId) ID() string {
	return string(n)
}

type dagStage struct {
	DagUID       string
	NodeId       string
	DependNodeId []string
	State        meta.StageState
	Code         string
	Variables    []string
	Connections  []string
}

func dagSort(dag *meta.Dag) ([]dagStage, error) {
	d := dagLib.NewDAG()
	nodeMap := make(map[string]meta.Node)
	for i, node := range dag.Nodes {
		if node.Code == "" {
			return nil, fmt.Errorf("dag %s node %s not code error", dag.UID, node.Id)
		}
		_, err := d.AddVertex(nodeId(node.Id))
		if err != nil {
			return nil, err
		}
		nodeMap[node.Id] = dag.Nodes[i]
	}
	for _, edge := range dag.Edges {
		err := d.AddEdge(edge.Source, edge.Target)
		if err != nil {
			return nil, err
		}
	}
	flog.Infof("dag %s: %s", dag.UID, d.String())

	roots := d.GetRoots()
	var result []dagStage
	for id, node := range nodeMap {
		parents, err := d.GetParents(id)
		if err != nil {
			return nil, err
		}
		var dependNodeId []string
		for pid := range parents {
			dependNodeId = append(dependNodeId, pid)
		}
		state := meta.StageCreate
		_, ok := roots[id]
		if ok {
			state = meta.StageReady
		}
		result = append(result, dagStage{
			DagUID:       dag.UID,
			NodeId:       id,
			DependNodeId: dependNodeId,
			State:        state,
			Code:         node.Code,
			Variables:    node.Variables,
			Connections:  node.Connections,
		})
	}

	return result, nil
}

// getJobFromTemplate2 makes a Job from a job. It converts the unix time into minutes from
// epoch time and concatenates that to the job name, because the dag_controller v2 has the lowest
// granularity of 1 minute for scheduling job.
func getStageFromTemplate(
	index int, cj *meta.Job, item dagStage,
	codes map[string]*meta.Code, variables map[string]*meta.Variable, connections map[string]*meta.Connection) (*meta.Stage, error) {

	// code
	code, ok := codes[item.Code]
	if !ok || code == nil {
		return nil, fmt.Errorf("not found code %s", item.Code)
	}

	// variables
	var stageVariables []meta.Variable
	for _, uid := range item.Variables {
		variable, ok := variables[uid]
		if !ok || variable == nil {
			return nil, fmt.Errorf("not found variable %s", uid)
		}
		stageVariables = append(stageVariables, *variable)
	}

	// connections
	var stageConnections []meta.Connection
	for _, uid := range item.Connections {
		connection, ok := connections[uid]
		if !ok || connection == nil {
			return nil, fmt.Errorf("not found connection %s", uid)
		}
		stageConnections = append(stageConnections, *connection)
	}

	name := getStageName(cj, index)
	now := time.Now()
	stage := &meta.Stage{
		TypeMeta: meta.TypeMeta{
			Kind:       "stage",
			APIVersion: constant.Version,
		},
		ObjectMeta: meta.ObjectMeta{
			Name:              name,
			CreationTimestamp: &now,
		},

		WorkflowUID: cj.WorkflowUID,
		JobUID:      cj.UID,
		DagUID:      item.DagUID,
		NodeID:      item.NodeId,

		State: item.State,

		Runtime:     code.Runtime,
		Code:        code.Code,
		Variables:   stageVariables,
		Connections: stageConnections,

		DependNodeId: item.DependNodeId,

		Input:  nil,
		Output: nil,
	}

	return stage, nil
}

func getStageName(cj *meta.Job, index int) string {
	return fmt.Sprintf("%s-%d", cj.UID, index)
}
