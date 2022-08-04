package worker

import (
	"errors"
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/apiserver/storage"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
	"net/http"
)

type WorkerStorage struct {
	REST *REST
}

func NewStorage(options *options.StoreOptions) (WorkerStorage, error) {
	r, err := NewREST(options)
	if err != nil {
		return WorkerStorage{}, err
	}
	return WorkerStorage{REST: r}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.Worker{} },
		NewListFunc:              func() runtime.Object { return &meta.WorkerList{} },
		NewStructFunc:            func() interface{} { return meta.Worker{} },
		NewListStructFunc:        func() interface{} { return meta.WorkerList{} },
		DefaultQualifiedResource: rest.Resource("worker"),

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
			Verb:         "PUT",
			SubResource:  "heartbeat",
			Params:       nil,
			ReadSample:   meta.Worker{},
			ReturnSample: meta.Status{},
		},
		{
			Verb:         "POST",
			SubResource:  "register",
			Params:       nil,
			ReadSample:   meta.Worker{},
			WriteSample:  meta.Worker{},
			ReturnSample: meta.Worker{},
		},
	}
}

func (r *REST) Handle(verb, subresource string, req *restful.Request, resp *restful.Response) {
	sr := &subResource{r}
	srRoute := rest.NewSubResourceRoute(verb, subresource, req, resp)
	srRoute.Match("PUT", "heartbeat", sr.heartbeat)
	srRoute.Match("POST", "register", sr.workerRegister)
	if !srRoute.Matched() {
		_ = resp.WriteError(http.StatusBadRequest, errors.New("error subresource path"))
	}
}

type subResource struct {
	store *REST
}

func (r *subResource) heartbeat(req *restful.Request, resp *restful.Response) {
	//ctx := req.Request.Context()
	obj := meta.Worker{}
	err := req.ReadEntity(&obj)
	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, errors.New("form error"))
		return
	}

	_ = resp.WriteEntity(meta.Status{Status: meta.StatusSuccess})
}

func (r *subResource) workerRegister(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	obj := meta.Worker{}
	err := req.ReadEntity(&obj)
	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, errors.New("form error"))
		return
	}

	obj.State = meta.WorkerReady

	find, err := r.store.Get(ctx, obj.UID, &meta.GetOptions{})
	if err != nil && !errors.Is(err, storage.ErrKeyNotFound) {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, errors.New("register error"))
		return
	}

	var result runtime.Object
	_, ok := find.(*meta.Worker)
	if ok {
		result, _, err = r.store.Update(ctx, obj.UID, &obj, rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc, false, &meta.UpdateOptions{})
	} else {
		result, err = r.store.Create(ctx, &obj, rest.ValidateAllObjectFunc, &meta.CreateOptions{})
	}

	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, errors.New("register error"))
		return
	}

	_ = resp.WriteEntity(result)
}
