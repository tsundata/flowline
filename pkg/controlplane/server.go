package controlplane

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/util/log"
	"net/http"
)

type GenericAPIServer struct {
	config  *Config
	Handler *APIServerHandler
}

func NewGenericAPIServer(name string, config *Config) *GenericAPIServer {
	handlerChainBuilder := func(handler http.Handler) http.Handler {
		return config.BuildHandlerChainFunc(handler, config)
	}
	apiServerHandler := NewAPIServerHandler(name, handlerChainBuilder, nil)
	s := &GenericAPIServer{
		config:  config,
		Handler: apiServerHandler,
	}

	installAPI(s, config)

	return s
}

func (g *GenericAPIServer) InstallAPIGroup() error {
	return nil
}

func (g *GenericAPIServer) Run(stopCh <-chan struct{}) error {
	addr := fmt.Sprintf("%s:%d", g.config.Host, g.config.Port)
	log.FLog.Info(fmt.Sprintf("apiserver addr %s", addr))
	if err := http.ListenAndServe(addr, g.Handler.NonRestfulMux); err != nil {
		return err
	}
	return nil
}
