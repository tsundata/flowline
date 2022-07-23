package handlers

import (
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"net/http"
)

func getHandler(s rest.Getter, scope *registry.RequestScope) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		ctx := req.Request.Context()
		uid := req.PathParameter("uid")
		out, err := s.Get(ctx, uid, &meta.GetOptions{}) // todo resourceVersion
		if err != nil {
			_ = resp.WriteError(http.StatusNotFound, err)
			return
		}
		_ = resp.WriteEntity(out)
	}
}

func GetResource(s rest.Getter, scope *registry.RequestScope) restful.RouteFunction {
	return getHandler(s, scope)
}

func listHandler(s rest.Lister, scope *registry.RequestScope) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		ctx := req.Request.Context()
		out, err := s.List(ctx, &meta.ListOptions{})
		if err != nil {
			_ = resp.WriteError(http.StatusInternalServerError, err)
			return
		}
		_ = resp.WriteEntity(out)
	}
}

func ListResource(s rest.Lister, scope *registry.RequestScope) restful.RouteFunction {
	return listHandler(s, scope)
}
