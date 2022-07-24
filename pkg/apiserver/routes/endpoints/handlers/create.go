package handlers

import (
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"net/http"
)

func createHandler(s rest.Creater, scope *registry.RequestScope) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		subResource, isSubResource := s.(rest.SubResourceStorage)
		if isSubResource {
			subResource.Handle(scope)
			return
		}
		ctx := req.Request.Context()
		obj := s.New()
		err := req.ReadEntity(&obj)
		if err != nil {
			_ = resp.WriteError(http.StatusInternalServerError, err)
			return
		}
		out, err := s.Create(ctx, obj, rest.ValidateAllObjectFunc, nil)
		if err != nil {
			_ = resp.WriteError(http.StatusInternalServerError, err)
			return
		}
		_ = resp.WriteEntity(out)
	}
}

func CreateResource(s rest.Creater, scope *registry.RequestScope) restful.RouteFunction {
	return createHandler(s, scope)
}
