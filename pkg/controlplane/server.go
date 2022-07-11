package controlplane

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/controlplane/registry/rest"
	"github.com/tsundata/flowline/pkg/controlplane/registry/rest/dag"
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
	storageMap := make(map[string]rest.Storage)
	dagStorage, _ := dag.NewREST()
	storageMap["dag"] = dagStorage // todo

	handlerChainBuilder := func(handler http.Handler) http.Handler {
		return config.BuildHandlerChainFunc(handler, config)
	}
	apiServerHandler := NewAPIServerHandler(name, handlerChainBuilder, nil)
	s := &GenericAPIServer{
		Handler: apiServerHandler,
		Storage: storageMap,
		httpServer: &http.Server{
			Addr:           fmt.Sprintf("%s:%d", config.Host, config.Port),
			Handler:        apiServerHandler.Director,
			ReadTimeout:    10 * time.Second, //todo
			WriteTimeout:   10 * time.Second, //todo
			MaxHeaderBytes: 1 << 20,          //todo
		},
	}

	err := installAPI(s, config)
	if err != nil {
		panic(err)
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
