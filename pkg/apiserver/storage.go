package apiserver

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/apiserver/config"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/code"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/connection"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/dag"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/event"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/job"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/role"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/rolebinding"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/stage"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/user"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/variable"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/worker"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/workflow"
	storageConfig "github.com/tsundata/flowline/pkg/apiserver/storage/config"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"github.com/tsundata/flowline/pkg/runtime/serializer/json"
	"github.com/tsundata/flowline/pkg/util/flog"
)

func StorageMap(config *config.Config) map[string]rest.Storage {
	storageMap := make(map[string]rest.Storage)
	codeRestStorage(config, storageMap)
	connectionRestStorage(config, storageMap)
	dagRestStorage(config, storageMap)
	eventRestStorage(config, storageMap)
	jobRestStorage(config, storageMap)
	roleRestStorage(config, storageMap)
	rolebindingRestStorage(config, storageMap)
	stageRestStorage(config, storageMap)
	userRestStorage(config, storageMap)
	variableRestStorage(config, storageMap)
	workerRestStorage(config, storageMap)
	workflowRestStorage(config, storageMap)
	return storageMap
}

func codeRestStorage(config *config.Config, storageMap map[string]rest.Storage) {
	s, err := code.NewREST(makeStoreOptions(config, "code"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["code"] = s
}

func connectionRestStorage(config *config.Config, storageMap map[string]rest.Storage) {
	s, err := connection.NewREST(makeStoreOptions(config, "connection"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["connection"] = s
}

func dagRestStorage(config *config.Config, storageMap map[string]rest.Storage) {
	s, err := dag.NewREST(makeStoreOptions(config, "dag"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["dag"] = s
}

func eventRestStorage(config *config.Config, storageMap map[string]rest.Storage) {
	s, err := event.NewREST(makeStoreOptions(config, "event"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["event"] = s
}

func jobRestStorage(config *config.Config, storageMap map[string]rest.Storage) {
	s, err := job.NewREST(makeStoreOptions(config, "job"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["job"] = s
}

func roleRestStorage(config *config.Config, storageMap map[string]rest.Storage) {
	s, err := role.NewREST(makeStoreOptions(config, "role"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["role"] = s
}

func rolebindingRestStorage(config *config.Config, storageMap map[string]rest.Storage) {
	s, err := rolebinding.NewREST(makeStoreOptions(config, "rolebinding"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["rolebinding"] = s
}

func stageRestStorage(config *config.Config, storageMap map[string]rest.Storage) {
	s, err := stage.NewREST(makeStoreOptions(config, "stage"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["stage"] = s
}

func userRestStorage(config *config.Config, storageMap map[string]rest.Storage) {
	s, err := user.NewREST(config, makeStoreOptions(config, "user"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["user"] = s
}

func variableRestStorage(config *config.Config, storageMap map[string]rest.Storage) {
	s, err := variable.NewREST(makeStoreOptions(config, "variable"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["variable"] = s
}

func workerRestStorage(config *config.Config, storageMap map[string]rest.Storage) {
	s, err := worker.NewREST(makeStoreOptions(config, "worker"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["worker"] = s
}

func workflowRestStorage(config *config.Config, storageMap map[string]rest.Storage) {
	s, err := workflow.NewREST(makeStoreOptions(config, "workflow"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["workflow"] = s
}

func makeStoreOptions(config *config.Config, resource string) *options.StoreOptions {
	codec := json.NewSerializerWithOptions(json.DefaultMetaFactory, json.SerializerOptions{})
	storeOptions := &options.StoreOptions{
		RESTOptions: &options.RESTOptions{
			StorageConfig: &storageConfig.ConfigForResource{
				Config: storageConfig.Config{
					Codec:     codec,
					Transport: config.ETCDConfig,
				},
				GroupResource: schema.GroupResource{
					Group:    constant.GroupName,
					Resource: resource,
				},
			},
			Decorator:               registry.StorageFactory(),
			EnableGarbageCollection: false,
			DeleteCollectionWorkers: 0,
			ResourcePrefix:          fmt.Sprintf("%s/%s/%s", constant.GroupName, constant.Version, resource),
			CountMetricPollPeriod:   0,
		},
	}

	return storeOptions
}
