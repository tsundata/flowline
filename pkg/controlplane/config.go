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

	JWTSecret string

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
func DefaultBuildHandlerChain(apiHandler http.Handler, config *Config) http.Handler {
	handler := filters.WithCORS(apiHandler, nil, nil, nil, nil, "true")
	// handler = filters.WithJWT(handler, config.JWTSecret)
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

	uidParam := ws.PathParameter("uid", "uid of the resource").DataType("string")

	getRoute := ws.GET(resource+"/{uid}").To(storage.GetHandler).
		Doc("Get resource").
		Metadata(restfulspec.KeyOpenAPITags, tags)
	getRoute.Param(uidParam)
	rs = append(rs, getRoute)

	postRoute := ws.POST(resource).To(storage.CreateHandler).
		Doc("Create resource").
		Metadata(restfulspec.KeyOpenAPITags, tags)
	rs = append(rs, postRoute)

	putRoute := ws.PUT(resource+"/{uid}").To(storage.UpdateHandler).
		Doc("Update resource").
		Metadata(restfulspec.KeyOpenAPITags, tags)
	putRoute.Param(uidParam)
	rs = append(rs, putRoute)

	deleteRoute := ws.DELETE(resource+"/{uid}").To(storage.DeleteHandler).
		Doc("Delete resource").
		Metadata(restfulspec.KeyOpenAPITags, tags)
	deleteRoute.Param(uidParam)
	rs = append(rs, deleteRoute)

	listRoute := ws.GET(resource+"/list").To(storage.ListHandler).
		Doc("List resource").
		Metadata(restfulspec.KeyOpenAPITags, tags)
	rs = append(rs, listRoute)

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
