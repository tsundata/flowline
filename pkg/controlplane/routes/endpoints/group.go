package endpoints

import (
	"github.com/emicklei/go-restful/v3"
)

func NewWebService(group string, version string) *restful.WebService {
	ws := new(restful.WebService)
	ws.Path("/apis/" + group + "/" + version)
	ws.Doc("API at /apis/apps/v1")
	ws.Consumes(restful.MIME_JSON)
	ws.Produces(restful.MIME_JSON)
	return ws
}
