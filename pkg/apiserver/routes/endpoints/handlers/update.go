package handlers

import (
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"net/http"
)

func updateHandler(s rest.Updater, scope *registry.RequestScope) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		subResource, isSubResource := s.(rest.SubResourceStorage)
		if isSubResource {
			subResource.Handle(scope)
			return
		}
		uid := req.PathParameter("uid")
		ctx := req.Request.Context()
		obj := s.New()
		err := req.ReadEntity(&obj)
		if err != nil {
			_ = resp.WriteError(http.StatusInternalServerError, err)
			return
		}
		out, _, err := s.Update(ctx, uid, obj, rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc, false, nil)
		if err != nil {
			_ = resp.WriteError(http.StatusInternalServerError, err)
			return
		}
		_ = resp.WriteEntity(out)
	}
}

func UpdateResource(s rest.Updater, scope *registry.RequestScope) restful.RouteFunction {
	return updateHandler(s, scope)
}
