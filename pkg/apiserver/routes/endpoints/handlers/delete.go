package handlers

import (
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"net/http"
)

func deleteHandler(s rest.Deleter, scope *registry.RequestScope) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		subResource, isSubResource := s.(rest.SubResourceStorage)
		if isSubResource && scope.Subresource != "" {
			subResource.Handle(scope.Verb, scope.Subresource, req, resp)
			return
		}
		uid := req.PathParameter("uid")
		ctx := req.Request.Context()
		out, _, err := s.Delete(ctx, uid, rest.ValidateAllObjectFunc, nil)
		if err != nil {
			_ = resp.WriteError(http.StatusInternalServerError, err)
			return
		}
		_ = resp.WriteEntity(out)
	}
}

func DeleteResource(s rest.Deleter, scope *registry.RequestScope) restful.RouteFunction {
	return deleteHandler(s, scope)
}
