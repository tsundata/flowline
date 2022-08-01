package job

import (
	"errors"
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
	"net/http"
)

type JobStorage struct {
	REST *REST
}

func NewStorage(options *options.StoreOptions) (JobStorage, error) {
	r, err := NewREST(options)
	if err != nil {
		return JobStorage{}, err
	}
	return JobStorage{REST: r}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.Job{} },
		NewListFunc:              func() runtime.Object { return &meta.JobList{} },
		NewStructFunc:            func() interface{} { return meta.Job{} },
		NewListStructFunc:        func() interface{} { return meta.JobList{} },
		DefaultQualifiedResource: rest.Resource("job"),

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
			SubResource:  "state",
			Params:       nil,
			ReadSample:   meta.Job{},
			ReturnSample: meta.Job{},
		},
	}
}

func (r *REST) Handle(verb, subresource string, req *restful.Request, resp *restful.Response) {
	sr := &subResource{r}
	srRoute := rest.NewSubResourceRoute(verb, subresource, req, resp)
	srRoute.Match("PUT", "state", sr.jobUpdateState)
	if !srRoute.Matched() {
		_ = resp.WriteError(http.StatusBadRequest, errors.New("error subresource path"))
	}
}

type subResource struct {
	store *REST
}

func (r *subResource) jobUpdateState(req *restful.Request, resp *restful.Response) {
	obj := meta.Job{}
	err := req.ReadEntity(&obj)
	if err != nil {
		flog.Error(err)
	}

	result, _, err := r.store.Update(req.Request.Context(), obj.UID, &obj, rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc, false, &meta.UpdateOptions{})

	_ = resp.WriteEntity(result)
}
