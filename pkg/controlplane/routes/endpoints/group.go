package endpoints

import (
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/controlplane/registry/rest"
)

func NewWebService(group string, version string) *restful.WebService {
	ws := new(restful.WebService)
	ws.Path("/apis/" + group + "/" + version)
	ws.Doc(fmt.Sprintf("API at /apis/%s/%s", rest.GroupName, rest.Version))
	ws.Consumes(restful.MIME_JSON)
	ws.Produces(restful.MIME_JSON)
	return ws
}
