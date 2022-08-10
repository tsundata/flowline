package event

import (
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
	"golang.org/x/xerrors"
	"net/http"
)

type EventStorage struct {
	REST *REST
}

func NewStorage(options *options.StoreOptions) (EventStorage, error) {
	r, err := NewREST(options)
	if err != nil {
		return EventStorage{}, err
	}
	return EventStorage{REST: r}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.Event{} },
		NewListFunc:              func() runtime.Object { return &meta.EventList{} },
		NewStructFunc:            func() interface{} { return meta.Event{} },
		NewListStructFunc:        func() interface{} { return meta.EventList{} },
		DefaultQualifiedResource: rest.Resource("event"),

		CreateStrategy:      Strategy,
		UpdateStrategy:      Strategy,
		DeleteStrategy:      Strategy,
		ResetFieldsStrategy: Strategy,

		ObjectUIDFunc: func(obj runtime.Object) (string, error) {
			accessor, err := meta.Accessor(obj)
			if err != nil {
				return "", err
			}
			if e, ok := obj.(*meta.Event); ok {
				return e.Name, nil
			}
			return accessor.GetUID(), nil
		},
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
			Verb:        "LIST",
			SubResource: "kind",
			Params: []*restful.Parameter{
				restful.QueryParameter("uid", "Involved Object UID").Required(true),
			},
			ReturnSample: meta.EventList{},
		},
	}
}

func (r *REST) Handle(verb, subresource string, req *restful.Request, resp *restful.Response) {
	sr := &subResource{r}
	srRoute := rest.NewSubResourceRoute(verb, subresource, req, resp)
	srRoute.Match("LIST", "kind", sr.eventListByUID)
	if !srRoute.Matched() {
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("error subresource path"))
	}
}

type subResource struct {
	store *REST
}

func (r *subResource) eventListByUID(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	uid := req.QueryParameter("uid")

	list := &meta.EventList{}
	err := r.store.Storage.GetList(ctx, rest.WithPrefix("event/"+uid), meta.ListOptions{}, list)
	if err != nil {
		flog.Error(err)
		_ = resp.WriteError(http.StatusBadRequest, xerrors.New("event list error"))
		return
	}

	_ = resp.WriteEntity(list)
}
