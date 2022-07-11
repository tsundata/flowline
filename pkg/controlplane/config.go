package controlplane

import (
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/controlplane/filters"
	"github.com/tsundata/flowline/pkg/controlplane/registry/rest"
	"github.com/tsundata/flowline/pkg/controlplane/routes"
	"github.com/tsundata/flowline/pkg/controlplane/routes/endpoints"
	"net/http"
	"sort"
)

const DefaultAPIPrefix = "/api"

type Config struct {
	Host string
	Port int

	EtcdHost     string
	EtcdUsername string
	EtcdPassword string

	// APIServerID is the ID of this API server
	APIServerID           string
	BuildHandlerChainFunc func(apiHandler http.Handler, c *Config) http.Handler

	EnableIndex bool
}

func NewConfig() *Config {
	return &Config{
		BuildHandlerChainFunc: DefaultBuildHandlerChain,
	}
}

//DefaultBuildHandlerChain set default filters
func DefaultBuildHandlerChain(apiHandler http.Handler, _ *Config) http.Handler {
	handler := filters.WithCORS(apiHandler, nil, nil, nil, nil, "true")
	return handler
}

//installAPI install routes
func installAPI(s *GenericAPIServer, c *Config) error {
	if c.EnableIndex {
		routes.Index{}.Install(s.Handler.NonRestfulMux)
	}
	return installAPIGroup(s, c)
}

func installAPIGroup(s *GenericAPIServer, _ *Config) error {
	ws := endpoints.NewWebService("apps", "v1")
	paths := make([]string, len(s.Storage))
	var i = 0
	for path := range s.Storage {
		paths[i] = path
		i++
	}
	sort.Strings(paths)
	for _, path := range paths {
		err := registerResourceHandlers(path, s.Storage[path], ws)
		if err != nil {
			return err
		}
	}

	s.Handler.RestfulContainer.Add(ws)
	return nil
}

func registerResourceHandlers(resource string, storage rest.Storage, ws *restful.WebService) error {
	var rs []*restful.RouteBuilder

	nameParam := ws.PathParameter("name", "name of the resource").DataType("string")

	getRoute := ws.GET(resource + "/{name}").To(storage.GetHandler)
	getRoute.Param(nameParam)
	rs = append(rs, getRoute)

	postRoute := ws.POST(resource).To(storage.PostHandler)
	rs = append(rs, postRoute)

	putRoute := ws.PUT(resource + "/{name}").To(storage.PutHandler)
	putRoute.Param(nameParam)
	rs = append(rs, putRoute)

	deleteRoute := ws.DELETE(resource + "/{name}").To(storage.DeleteHandler)
	deleteRoute.Param(nameParam)
	rs = append(rs, deleteRoute)

	for _, route := range rs {
		ws.Route(route)
	}

	return nil
}
