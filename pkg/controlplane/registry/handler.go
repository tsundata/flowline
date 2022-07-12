package registry

import (
	"context"
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"net/http"
	"time"
)

func (e *Store) GetHandler(req *restful.Request, resp *restful.Response) {
	name := req.PathParameter("name")
	_ = resp.WriteEntity(fmt.Sprintf("get response %s %+v", name, e.New()))
}

func (e *Store) PostHandler(req *restful.Request, resp *restful.Response) {
	ctx := context.Background()
	obj := e.New()
	err := req.ReadEntity(&obj)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	//key, err := e.KeyFunc(ctx, "dag")
	//if err != nil {
	//	resp.WriteError(http.StatusInternalServerError, err)
	//	return
	//}
	key := e.KeyRootFunc(ctx)
	e.Storage.Create(ctx, key+time.Now().String(), obj, obj, 0, false)
	_ = resp.WriteEntity(obj)
}

func (e *Store) PutHandler(req *restful.Request, resp *restful.Response) {
	name := req.PathParameter("name")
	_ = resp.WriteEntity(fmt.Sprintf("put response %s", name))
}

func (e *Store) DeleteHandler(req *restful.Request, resp *restful.Response) {
	name := req.PathParameter("name")
	_ = resp.WriteEntity(fmt.Sprintf("delete response %s", name))
}
