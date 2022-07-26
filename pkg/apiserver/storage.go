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
	codeRestStorage(storageMap)
	connectionRestStorage(storageMap)
	dagRestStorage(storageMap)
	eventRestStorage(storageMap)
	jobRestStorage(storageMap)
	roleRestStorage(storageMap)
	rolebindingRestStorage(storageMap)
	stageRestStorage(storageMap)
	userRestStorage(config, storageMap)
	variableRestStorage(storageMap)
	workerRestStorage(storageMap)
	workflowRestStorage(storageMap)
	return storageMap
}

func codeRestStorage(storageMap map[string]rest.Storage) {
	s, err := code.NewREST(makeStoreOptions("code"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["code"] = s
}

func connectionRestStorage(storageMap map[string]rest.Storage) {
	s, err := connection.NewREST(makeStoreOptions("connection"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["connection"] = s
}

func dagRestStorage(storageMap map[string]rest.Storage) {
	s, err := dag.NewREST(makeStoreOptions("dag"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["dag"] = s
}

func eventRestStorage(storageMap map[string]rest.Storage) {
	s, err := event.NewREST(makeStoreOptions("event"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["event"] = s
}

func jobRestStorage(storageMap map[string]rest.Storage) {
	s, err := job.NewREST(makeStoreOptions("job"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["job"] = s
}

func roleRestStorage(storageMap map[string]rest.Storage) {
	s, err := role.NewREST(makeStoreOptions("role"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["role"] = s
}

func rolebindingRestStorage(storageMap map[string]rest.Storage) {
	s, err := rolebinding.NewREST(makeStoreOptions("rolebinding"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["rolebinding"] = s
}

func stageRestStorage(storageMap map[string]rest.Storage) {
	s, err := stage.NewREST(makeStoreOptions("stage"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["stage"] = s
}

func userRestStorage(config *config.Config, storageMap map[string]rest.Storage) {
	s, err := user.NewREST(config, makeStoreOptions("user"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["user"] = s
}

func variableRestStorage(storageMap map[string]rest.Storage) {
	s, err := variable.NewREST(makeStoreOptions("variable"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["variable"] = s
}

func workerRestStorage(storageMap map[string]rest.Storage) {
	s, err := worker.NewREST(makeStoreOptions("worker"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["worker"] = s
}

func workflowRestStorage(storageMap map[string]rest.Storage) {
	s, err := workflow.NewREST(makeStoreOptions("workflow"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["workflow"] = s
}

func makeStoreOptions(resource string) *options.StoreOptions {
	codec := json.NewSerializerWithOptions(json.DefaultMetaFactory, json.SerializerOptions{})
	storeOptions := &options.StoreOptions{
		RESTOptions: &options.RESTOptions{
			StorageConfig: &storageConfig.ConfigForResource{
				Config: storageConfig.Config{
					Codec: codec,
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
