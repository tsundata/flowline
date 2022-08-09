package workflow

import (
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/util/flog"
	"golang.org/x/xerrors"
	"net/http"
	"time"
)

type WorkflowStorage struct {
	REST *REST
}

func NewStorage(options *options.StoreOptions) (WorkflowStorage, error) {
	r, err := NewREST(options)
	if err != nil {
		return WorkflowStorage{}, err
	}
	return WorkflowStorage{REST: r}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.Workflow{} },
		NewListFunc:              func() runtime.Object { return &meta.WorkflowList{} },
		NewStructFunc:            func() interface{} { return meta.Workflow{} },
		NewListStructFunc:        func() interface{} { return meta.WorkflowList{} },
		DefaultQualifiedResource: rest.Resource("workflow"),

		CreateStrategy:      Strategy,
		UpdateStrategy:      Strategy,
		DeleteStrategy:      Strategy,
		ResetFieldsStrategy: Strategy,
	}

	err := store.CompleteWithOptions(options)
	if err != nil {
		flog.Panic(err)
	}

	return &REST{store}, nil
}

func (r *REST) Actions() []rest.SubResourceAction {
	return []rest.SubResourceAction{
		{
			Verb:        "GET",
			SubResource: "dag",
			Params: []*restful.Parameter{
				restful.QueryParameter("jobUID", "Job UID for query status"),
			},
			ReadSample:   meta.Dag{},
			WriteSample:  meta.Dag{},
			ReturnSample: meta.Dag{},
		},
		{
			Verb:         "PUT",
			SubResource:  "dag",
			Params:       nil,
			ReadSample:   meta.Dag{},
			ReturnSample: meta.Status{},
		},
		{
			Verb:         "PUT",
			SubResource:  "state",
			Params:       nil,
			ReadSample:   meta.Workflow{},
			ReturnSample: meta.Workflow{},
		},
		{
			Verb:         "POST",
			SubResource:  "schedule",
			Params:       nil,
			ReadSample:   meta.Workflow{},
			ReturnSample: meta.Job{},
		},
	}
}

func (r *REST) Handle(verb, subresource string, req *restful.Request, resp *restful.Response) {
	sr := &subResource{r}
	srRoute := rest.NewSubResourceRoute(verb, subresource, req, resp)
	srRoute.Match("GET", "dag", sr.workflowGetDag)
	srRoute.Match("PUT", "dag", sr.workflowUpdateDag)
	srRoute.Match("PUT", "state", sr.workflowUpdateState)
	srRoute.Match("POST", "schedule", sr.workflowScheduleNow)
	if !srRoute.Matched() {
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("error subresource path"))
	}
}

type subResource struct {
	store *REST
}

func (r *subResource) workflowGetDag(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	u := req.PathParameter("uid")

	list := &meta.DagList{}
	err := r.store.Storage.GetList(ctx, rest.WithPrefix("dag"), meta.ListOptions{}, list)
	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("dag error"))
		return
	}

	var dag *meta.Dag
	for i, item := range list.Items {
		if item.WorkflowUID == u {
			dag = &list.Items[i]
			break
		}
	}
	if dag == nil {
		_ = resp.WriteError(http.StatusNotFound, xerrors.New("dag not found"))
		return
	}

	jobUID := req.QueryParameter("jobUID")
	if jobUID != "" {
		stageList := &meta.StageList{}
		err = r.store.Storage.GetList(ctx, rest.WithPrefix("stage"), meta.ListOptions{}, stageList)
		if err != nil || stageList == nil {
			_ = resp.WriteError(http.StatusNotFound, xerrors.New("stage not found"))
			return
		}
		var stages []*meta.Stage
		for i, item := range stageList.Items {
			if item.JobUID == jobUID {
				stages = append(stages, &stageList.Items[i])
			}
		}
		nodeState := make(map[string]meta.StageState)
		for _, stage := range stages {
			nodeState[stage.NodeID] = stage.State
		}

		for i, node := range dag.Nodes {
			if s, ok := nodeState[node.Id]; ok {
				switch s {
				case meta.StageSuccess:
					dag.Nodes[i].Status = meta.NodeSuccess
				case meta.StageFailed:
					dag.Nodes[i].Status = meta.NodeError
				case meta.StageBind:
					dag.Nodes[i].Status = meta.NodeProcessing
				default:
					dag.Nodes[i].Status = meta.NodeDefault
				}
			}
		}
	}

	_ = resp.WriteEntity(dag)
}

func (r *subResource) workflowUpdateDag(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	workflowUID := req.PathParameter("uid")

	obj := meta.Dag{}
	err := req.ReadEntity(&obj)
	if err != nil {
		flog.Error(err)
	}

	// query created dag
	list := &meta.DagList{}
	err = r.store.Storage.GetList(ctx, rest.WithPrefix("dag"), meta.ListOptions{}, list)
	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("dag list error"))
		return
	}
	var dag *meta.Dag
	for i, item := range list.Items {
		if item.WorkflowUID == workflowUID {
			dag = &list.Items[i]
			break
		}
	}

	if dag != nil {
		// update
		obj.WorkflowUID = workflowUID
		obj.UID = dag.UID
		err = r.store.Storage.GuaranteedUpdate(ctx, rest.WithPrefix("dag/"+dag.UID), &obj, false, nil, nil, false, nil)
	} else {
		// create
		obj.WorkflowUID = workflowUID
		rest.FillObjectMetaSystemFields(&obj)
		err = r.store.Storage.Create(ctx, rest.WithPrefix("dag/"+obj.UID), &obj, &obj, 0, false)
	}
	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("dag error"))
		return
	}

	_ = resp.WriteEntity(meta.Status{Status: meta.StatusSuccess})
}

func (r *subResource) workflowUpdateState(req *restful.Request, resp *restful.Response) {
	obj := meta.Workflow{}
	err := req.ReadEntity(&obj)
	if err != nil {
		flog.Error(err)
	}

	result, _, err := r.store.Update(req.Request.Context(), obj.UID, &obj, rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc, false, &meta.UpdateOptions{})
	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("workflow update state error"))
		return
	}

	_ = resp.WriteEntity(result)
}

func (r *subResource) workflowScheduleNow(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	workflow := meta.Workflow{}
	err := req.ReadEntity(&workflow)
	if err != nil {
		flog.Error(err)
	}
	workflowUID := workflow.UID

	job := &meta.Job{
		TypeMeta: meta.TypeMeta{
			Kind:       "job",
			APIVersion: constant.Version,
		},
		ObjectMeta: meta.ObjectMeta{
			Name: fmt.Sprintf("%v.%x", workflowUID, time.Now().UnixNano()),
		},
		WorkflowUID: workflowUID,
		State:       meta.JobCreate,
	}
	rest.FillObjectMetaSystemFields(job)

	err = r.store.Storage.Create(ctx, rest.WithPrefix(fmt.Sprintf("job/%s", job.UID)), job, job, 0, false)
	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("workflow create job error"))
		return
	}

	_ = resp.WriteEntity(job)
}
