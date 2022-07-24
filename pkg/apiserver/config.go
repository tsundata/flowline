package apiserver

import (
	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/go-openapi/spec"
	"github.com/tsundata/flowline/pkg/apiserver/filters"
	"github.com/tsundata/flowline/pkg/apiserver/routes"
	"github.com/tsundata/flowline/pkg/apiserver/routes/endpoints"
	"net/http"
)

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
	if ss, ok := s.Storage["worker"]; ok {
		routes.Worker{Storage: ss}.Install(s.Handler.NonRestfulMux)
	}

	installer := endpoints.NewAPIInstaller(s.Storage)
	ws, err := installer.Install()
	if err != nil {
		return err
	}
	s.Handler.RestfulContainer.Add(ws)

	return installAPISwagger(s)
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
					URL:   "https://flowline.tsundata.com",
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
