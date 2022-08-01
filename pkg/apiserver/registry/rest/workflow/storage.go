package workflow

import (
	"errors"
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/uid"
	"net/http"
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
			Verb:         "GET",
			SubResource:  "dag",
			Params:       nil,
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
	}
}

func (r *REST) Handle(verb, subresource string, req *restful.Request, resp *restful.Response) {
	sr := &subResource{r}
	srRoute := rest.NewSubResourceRoute(verb, subresource, req, resp)
	srRoute.Match("GET", "dag", sr.workflowGetDag)
	srRoute.Match("PUT", "dag", sr.workflowUpdateDag)
	srRoute.Match("PUT", "state", sr.workflowUpdateState)
	if !srRoute.Matched() {
		_ = resp.WriteError(http.StatusBadRequest, errors.New("error subresource path"))
	}
}

type subResource struct {
	store *REST
}

func (r *subResource) workflowGetDag(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	u := req.PathParameter("uid")

	list := &meta.DagList{}
	err := r.store.Storage.GetList(ctx, fmt.Sprintf("/%s/%s/dag", constant.GroupName, constant.Version), meta.ListOptions{}, list)
	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, errors.New("dag error"))
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
		_ = resp.WriteError(http.StatusNotFound, errors.New("dag not found"))
		return
	}

	_ = resp.WriteEntity(dag)
	return
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
	err = r.store.Storage.GetList(ctx, fmt.Sprintf("/%s/%s/dag", constant.GroupName, constant.Version), meta.ListOptions{}, list)
	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, errors.New("dag error"))
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
		err = r.store.Storage.GuaranteedUpdate(ctx, fmt.Sprintf("/%s/%s/dag/%s", constant.GroupName, constant.Version, dag.UID), &obj, false, nil, nil, false, nil)
	} else {
		// create
		obj.WorkflowUID = workflowUID
		obj.UID = uid.New()
		err = r.store.Storage.Create(ctx, fmt.Sprintf("/%s/%s/dag/%s", constant.GroupName, constant.Version, obj.UID), &obj, &obj, 0, false)
	}
	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, errors.New("dag error"))
		return
	}

	_ = resp.WriteEntity(meta.Status{Status: meta.StatusSuccess})
	return
}

func (r *subResource) workflowUpdateState(req *restful.Request, resp *restful.Response) {
	obj := meta.Workflow{}
	err := req.ReadEntity(&obj)
	if err != nil {
		flog.Error(err)
	}

	result, _, err := r.store.Update(req.Request.Context(), obj.UID, &obj, rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc, false, &meta.UpdateOptions{})

	_ = resp.WriteEntity(result)
}
