package stage

import (
	"errors"
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
	"net/http"
)

type StageStorage struct {
	REST *REST
}

func NewStorage(options *options.StoreOptions) (StageStorage, error) {
	r, err := NewREST(options)
	if err != nil {
		return StageStorage{}, err
	}
	return StageStorage{REST: r}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.Stage{} },
		NewListFunc:              func() runtime.Object { return &meta.StageList{} },
		NewStructFunc:            func() interface{} { return meta.Stage{} },
		NewListStructFunc:        func() interface{} { return meta.StageList{} },
		DefaultQualifiedResource: rest.Resource("stage"),

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
			Verb:         "POST",
			SubResource:  "binding",
			Params:       nil,
			ReadSample:   meta.Binding{},
			WriteSample:  meta.Binding{},
			ReturnSample: meta.Binding{},
		},
	}
}

func (r *REST) Handle(verb, subresource string, req *restful.Request, resp *restful.Response) {
	srRoute := rest.NewSubResourceRoute(verb, subresource, req, resp)
	srRoute.Match("POST", "binding", stageBinding)
	if !srRoute.Matched() {
		_ = resp.WriteError(http.StatusBadRequest, errors.New("error subresource path"))
	}
}

func stageBinding(req *restful.Request, resp *restful.Response) {
	obj := meta.Binding{}
	err := req.ReadEntity(&obj)
	if err != nil {
		flog.Error(err)
	}
	fmt.Printf("%+v \n", obj)
	return
}
