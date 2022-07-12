package registry

import (
	"context"
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/controlplane/storage"
	"net/http"
)

func (e *Store) GetHandler(req *restful.Request, resp *restful.Response) {
	ctx := context.Background()
	uid := req.PathParameter("uid")
	out, err := e.Get(ctx, uid, &storage.GetOptions{}) // todo resourceVersion
	if err != nil {
		_ = resp.WriteError(http.StatusNotFound, err)
		return
	}
	_ = resp.WriteEntity(out)
}

func (e *Store) PostHandler(req *restful.Request, resp *restful.Response) {
	ctx := context.Background()
	obj := e.New()
	err := req.ReadEntity(&obj)
	if err != nil {
		_ = resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	out, err := e.Create(ctx, obj, nil, nil)
	if err != nil {
		_ = resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	_ = resp.WriteEntity(out)
}

func (e *Store) PutHandler(req *restful.Request, resp *restful.Response) {
	uid := req.PathParameter("uid")
	ctx := context.Background()
	obj := e.New()
	err := req.ReadEntity(&obj)
	if err != nil {
		_ = resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	out, _, err := e.Update(ctx, uid, obj, nil, nil, true, nil)
	if err != nil {
		_ = resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	_ = resp.WriteEntity(out)
}

func (e *Store) DeleteHandler(req *restful.Request, resp *restful.Response) {
	uid := req.PathParameter("uid")
	ctx := context.Background()
	out, _, err := e.Delete(ctx, uid, nil, nil)
	if err != nil {
		_ = resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	_ = resp.WriteEntity(out)
}
