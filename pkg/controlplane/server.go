package controlplane

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/util/log"
	"net/http"
	"time"
)

type GenericAPIServer struct {
	Handler    *APIServerHandler
	httpServer *http.Server
}

func NewGenericAPIServer(name string, config *Config) *GenericAPIServer {
	handlerChainBuilder := func(handler http.Handler) http.Handler {
		return config.BuildHandlerChainFunc(handler, config)
	}
	apiServerHandler := NewAPIServerHandler(name, handlerChainBuilder, nil)
	s := &GenericAPIServer{
		Handler: apiServerHandler,
		httpServer: &http.Server{
			Addr:           fmt.Sprintf("%s:%d", config.Host, config.Port),
			Handler:        apiServerHandler.Director,
			ReadTimeout:    10 * time.Second, //todo
			WriteTimeout:   10 * time.Second, //todo
			MaxHeaderBytes: 1 << 20,          //todo
		},
	}

	installAPI(s, config)

	return s
}

func (g *GenericAPIServer) Run(stopCh <-chan struct{}) error {
	log.FLog.Info(fmt.Sprintf("apiserver addr %s", g.httpServer.Addr))
	if err := g.httpServer.ListenAndServe(); err != nil {
		return err
	}
	return nil
}
