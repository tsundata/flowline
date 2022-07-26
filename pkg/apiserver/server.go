package apiserver

import (
	"github.com/tsundata/flowline/pkg/apiserver/config"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/util/flog"
	"net"
	"net/http"
	"strconv"
)

type GenericAPIServer struct {
	Handler    *APIServerHandler
	Storage    map[string]rest.Storage
	httpServer *http.Server
}

func NewGenericAPIServer(name string, config *config.Config) *GenericAPIServer {
	handlerChainBuilder := func(handler http.Handler) http.Handler {
		return config.BuildHandlerChainFunc(handler, config)
	}
	apiServerHandler := NewAPIServerHandler(name, handlerChainBuilder, nil)
	s := &GenericAPIServer{
		Handler: apiServerHandler,
		Storage: StorageMap(config),
		httpServer: &http.Server{
			Addr:           net.JoinHostPort(config.Host, strconv.Itoa(config.Port)),
			Handler:        apiServerHandler,
			ReadTimeout:    config.HTTPReadTimeout,
			WriteTimeout:   config.HTTPWriteTimeout,
			MaxHeaderBytes: config.HTTPMaxHeaderBytes,
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
