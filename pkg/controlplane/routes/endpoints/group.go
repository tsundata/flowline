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

func RegisterHandler(resource string, ws *restful.WebService) {
	var routes []*restful.RouteBuilder

	nameParam := ws.PathParameter("name", "name of the resource").DataType("string")
	namespaceParam := ws.PathParameter("namespace", "object name and auth scope, such as for teams and projects").DataType("string")

	route := ws.GET("namespaces" + "/{namespace}/" + resource + "/{name}").To(getHandler)
	route.Param(namespaceParam)
	route.Param(nameParam)
	route.Writes(Foo{})
	routes = append(routes, route)

	route2 := ws.POST("namespaces" + "/{namespace}/" + resource).To(postHandler)
	route2.Param(namespaceParam)
	routes = append(routes, route2)

	for _, route := range routes {
		ws.Route(route)
	}
}

type Foo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func getHandler(req *restful.Request, res *restful.Response) {
	namespace := req.PathParameter("namespace")
	name := req.PathParameter("name")
	_ = res.WriteEntity(Foo{Namespace: namespace, Name: name})
}

func postHandler(req *restful.Request, res *restful.Response) {
	namespace := req.PathParameter("namespace")
	_ = res.WriteEntity(Foo{Namespace: namespace, Name: ""})
}
