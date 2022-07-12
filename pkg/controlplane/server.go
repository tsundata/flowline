package controlplane

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/controlplane/registry"
	"github.com/tsundata/flowline/pkg/controlplane/registry/options"
	"github.com/tsundata/flowline/pkg/controlplane/registry/rest"
	"github.com/tsundata/flowline/pkg/controlplane/registry/rest/dag"
	config2 "github.com/tsundata/flowline/pkg/controlplane/storage/config"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
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
	jsonCoder := runtime.JsonCoder{}
	codec := runtime.NewBase64Serializer(jsonCoder, jsonCoder)
	storeOptions := &options.StoreOptions{
		RESTOptions: &options.RESTOptions{
			StorageConfig: &config2.ConfigForResource{
				Config: config2.Config{
					Codec: codec,
				},
				GroupResource: schema.GroupResource{
					Group:    "apps",
					Resource: "dag",
				},
			},
			Decorator:               registry.StorageFactory(),
			EnableGarbageCollection: false,
			DeleteCollectionWorkers: 0,
			ResourcePrefix:          "dag",
			CountMetricPollPeriod:   0,
		},
	}

	storageMap := make(map[string]rest.Storage)
	dagStorage, _ := dag.NewREST(storeOptions)
	storageMap["dag"] = dagStorage // fixme

	handlerChainBuilder := func(handler http.Handler) http.Handler {
		return config.BuildHandlerChainFunc(handler, config)
	}
	apiServerHandler := NewAPIServerHandler(name, handlerChainBuilder, nil)
	s := &GenericAPIServer{
		Handler: apiServerHandler,
		Storage: storageMap,
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
