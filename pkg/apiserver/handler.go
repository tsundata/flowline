package apiserver

import (
	"bytes"
	"fmt"
	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/go-openapi/spec"
	"github.com/gorilla/mux"
	"github.com/tsundata/flowline/pkg/apiserver/config"
	"github.com/tsundata/flowline/pkg/apiserver/filters"
	"github.com/tsundata/flowline/pkg/apiserver/routes"
	"github.com/tsundata/flowline/pkg/apiserver/routes/endpoints"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/util/flog"
	"golang.org/x/xerrors"
	"net/http"
	"runtime"
	"strings"
)

type APIServerHandler struct {
	FullHandlerChain http.Handler
	RestfulContainer *restful.Container
	NonRestfulMux    *mux.Router

	Director http.Handler
}

type HandlerChainBuilderFn func(apiHandler http.Handler) http.Handler

func NewAPIServerHandler(name string, handlerChainBuilder HandlerChainBuilderFn, notFoundHandler http.Handler) *APIServerHandler {
	nonRestfulMux := mux.NewRouter()
	if notFoundHandler != nil {
		nonRestfulMux.NotFoundHandler = notFoundHandler
	}

	restfulContainer := restful.NewContainer()
	restfulContainer.ServeMux = http.NewServeMux()
	restfulContainer.Router(restful.CurlyRouter{})
	restfulContainer.RecoverHandler(func(panicReason interface{}, httpWriter http.ResponseWriter) {
		logStackOnRecover(panicReason, httpWriter)
	})
	restfulContainer.ServiceErrorHandler(func(serviceError restful.ServiceError, req *restful.Request, resp *restful.Response) {
		serviceErrorHandler(serviceError, req, resp)
	})

	d := director{
		name:             name,
		restfulContainer: restfulContainer,
		nonRestfulMux:    nonRestfulMux,
	}

	return &APIServerHandler{
		FullHandlerChain: handlerChainBuilder(d),
		RestfulContainer: restfulContainer,
		NonRestfulMux:    nonRestfulMux,
		Director:         d,
	}
}

type director struct {
	name             string
	restfulContainer *restful.Container
	nonRestfulMux    *mux.Router
}

func (d director) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	flog.Infof("%s %s", req.Method, path)

	for _, ws := range d.restfulContainer.RegisteredWebServices() {
		switch {
		case ws.RootPath() == "/"+constant.RestPrefix:
			if path == "/"+constant.RestPrefix || path == "/"+constant.RestPrefix+"/" {
				d.restfulContainer.Dispatch(w, req)
				return
			}
		case strings.HasPrefix(path, ws.RootPath()):
			if len(path) == len(ws.RootPath()) || path[len(ws.RootPath())] == '/' {
				d.restfulContainer.Dispatch(w, req)
				return
			}
		}
	}

	d.nonRestfulMux.ServeHTTP(w, req)
}

func logStackOnRecover(panicReason interface{}, w http.ResponseWriter) {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("recover from panic situation: - %v\r\n", panicReason))
	for i := 2; ; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		buffer.WriteString(fmt.Sprintf("    %s:%d\r\n", file, line))
	}
	flog.Error(xerrors.New(buffer.String()))

	headers := http.Header{}
	if ct := w.Header().Get("Content-Type"); len(ct) > 0 {
		headers.Set("Accept", ct)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write(buffer.Bytes())
}

func serviceErrorHandler(serviceError restful.ServiceError, _ *restful.Request, resp *restful.Response) {
	errText := fmt.Sprintf("error %d %s", serviceError.Code, serviceError.Message)
	flog.Error(xerrors.New(errText))

	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(http.StatusInternalServerError)
	_, _ = resp.Write([]byte(errText))
}

func (a *APIServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.FullHandlerChain.ServeHTTP(w, r)
}

// DefaultBuildHandlerChain set default filters
func DefaultBuildHandlerChain(apiHandler http.Handler, config *config.Config) http.Handler {
	handler := apiHandler
	authWhiteList := []string{
		"/swagger.json",
		"/api/apps/v1/user/session",
	}
	handler = filters.WithRBAC(handler, config.Storage, authWhiteList)
	handler = filters.WithJWT(handler, config.JWTSecret, authWhiteList)
	handler = filters.WithCORS(handler, config.CorsAllowedOriginPatterns, nil, nil, nil, "true")
	return handler
}

// installAPI install routes
func installAPI(s *GenericAPIServer, c *config.Config) error {
	if c.EnableIndex {
		routes.Index{}.Install(s.Handler.NonRestfulMux)
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
	c := restfulspec.Config{
		WebServices:                   s.Handler.RestfulContainer.RegisteredWebServices(),
		APIPath:                       "/swagger.json",
		PostBuildSwaggerObjectHandler: enrichSwaggerObject,
	}
	s.Handler.RestfulContainer.Add(restfulspec.NewOpenAPIService(c))
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
	swo.SecurityDefinitions = map[string]*spec.SecurityScheme{
		"BearerToken": {
			SecuritySchemeProps: spec.SecuritySchemeProps{
				Type:        "apiKey",
				Name:        "authorization",
				In:          "header",
				Description: "Bearer Token authentication",
			},
		},
	}
	swo.Security = []map[string][]string{
		{
			"BearerToken": {},
		},
	}
}
