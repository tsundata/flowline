package controlplane

import (
	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/go-openapi/spec"
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
	err := installAPIGroup(s, c)
	if err != nil {
		return err
	}
	return installAPISwagger(s)
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
	tags := []string{resource}
	var rs []*restful.RouteBuilder

	nameParam := ws.PathParameter("name", "name of the resource").DataType("string")

	getRoute := ws.GET(resource+"/{name}").To(storage.GetHandler).
		Doc("Get resource").
		Metadata(restfulspec.KeyOpenAPITags, tags)
	getRoute.Param(nameParam)
	rs = append(rs, getRoute)

	postRoute := ws.POST(resource).To(storage.PostHandler).
		Doc("Create resource").
		Metadata(restfulspec.KeyOpenAPITags, tags)
	rs = append(rs, postRoute)

	putRoute := ws.PUT(resource+"/{name}").To(storage.PutHandler).
		Doc("Update resource").
		Metadata(restfulspec.KeyOpenAPITags, tags)
	putRoute.Param(nameParam)
	rs = append(rs, putRoute)

	deleteRoute := ws.DELETE(resource+"/{name}").To(storage.DeleteHandler).
		Doc("Delete resource").
		Metadata(restfulspec.KeyOpenAPITags, tags)
	deleteRoute.Param(nameParam)
	rs = append(rs, deleteRoute)

	for _, route := range rs {
		ws.Route(route)
	}

	return nil
}

func installAPISwagger(s *GenericAPIServer) error {
	config := restfulspec.Config{
		WebServices:                   s.Handler.RestfulContainer.RegisteredWebServices(),
		APIPath:                       "/apidocs.json",
		PostBuildSwaggerObjectHandler: enrichSwaggerObject,
	}
	s.Handler.RestfulContainer.Add(restfulspec.NewOpenAPIService(config))
	return nil
}

func enrichSwaggerObject(swo *spec.Swagger) {
	swo.Info = &spec.Info{
		InfoProps: spec.InfoProps{
			Title:       "Flowline REST API",
			Description: "Resource for flowline",
			Contact: &spec.ContactInfo{
				ContactInfoProps: spec.ContactInfoProps{
					Name:  "sysatom",
					Email: "sysatom@gmail.com",
					URL:   "http://flowline.tsundata.com",
				},
			},
			License: &spec.License{
				LicenseProps: spec.LicenseProps{
					Name: "MIT",
					URL:  "https://github.com/tsundata/flowline/blob/main/LICENSE",
				},
			},
			Version: "1.0.0",
		},
	}
	swo.Tags = []spec.Tag{{TagProps: spec.TagProps{
		Name:        "workflows",
		Description: "Managing workflows"}}}
}
