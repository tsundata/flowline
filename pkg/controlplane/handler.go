package controlplane

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"github.com/gorilla/mux"
	"github.com/tsundata/flowline/pkg/util/log"
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
	log.FLog.Info(fmt.Sprintf("%s %s", req.Method, path))

	for _, ws := range d.restfulContainer.RegisteredWebServices() {
		switch {
		case ws.RootPath() == "/apis":
			if path == "/apis" || path == "/apis/" {
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
	log.FLog.Error(errors.New(buffer.String()))

	headers := http.Header{}
	if ct := w.Header().Get("Content-Type"); len(ct) > 0 {
		headers.Set("Accept", ct)
	}
	w.Header().Set("Content-Type", "application/json") //todo
	w.WriteHeader(http.StatusInternalServerError)      //todo
	w.Write(buffer.Bytes())                            //todo
}

func serviceErrorHandler(serviceError restful.ServiceError, _ *restful.Request, resp *restful.Response) {
	errText := fmt.Sprintf("error %d %s", serviceError.Code, serviceError.Message)
	log.FLog.Error(errors.New(errText))

	resp.Header().Set("Content-Type", "application/json") //todo
	resp.WriteHeader(http.StatusInternalServerError)      //todo
	resp.Write([]byte(errText))                           //todo
}
