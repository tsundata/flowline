package apiserver

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/util/flog"
	"net/http"
	"time"
)

type GenericAPIServer struct {
	Handler    *APIServerHandler
	Storage    map[string]rest.Storage
	httpServer *http.Server
}

func NewGenericAPIServer(name string, config *Config) *GenericAPIServer {
	handlerChainBuilder := func(handler http.Handler) http.Handler {
		return config.BuildHandlerChainFunc(handler, config)
	}
	apiServerHandler := NewAPIServerHandler(name, handlerChainBuilder, nil)
	s := &GenericAPIServer{
		Handler: apiServerHandler,
		Storage: StorageMap(),
		httpServer: &http.Server{
			Addr:           fmt.Sprintf("%s:%d", config.Host, config.Port),
			Handler:        apiServerHandler,
			ReadTimeout:    10 * time.Second, //todo
			WriteTimeout:   10 * time.Second, //todo
			MaxHeaderBytes: 1 << 20,          //todo
		},
	}

	err := installAPI(s, config)
	if err != nil {
		flog.Panic(err)
	}

	return s
}

func (g *GenericAPIServer) Run(_ <-chan struct{}) error {
	flog.Infof("apiserver addr %s", g.httpServer.Addr)
	if err := g.httpServer.ListenAndServe(); err != nil {
		return err
	}
	return nil
}
