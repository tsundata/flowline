package apiserver

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/connection"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/dag"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/event"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/function"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/job"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/role"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/rolebinding"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/user"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/variable"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/worker"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest/workflow"
	"github.com/tsundata/flowline/pkg/apiserver/storage/config"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"github.com/tsundata/flowline/pkg/util/flog"
)

func StorageMap() map[string]rest.Storage {
	storageMap := make(map[string]rest.Storage)
	connectionRestStorage(storageMap)
	dagRestStorage(storageMap)
	eventRestStorage(storageMap)
	functionRestStorage(storageMap)
	jobRestStorage(storageMap)
	roleRestStorage(storageMap)
	rolebindingRestStorage(storageMap)
	stageRestStorage(storageMap)
	userRestStorage(storageMap)
	variableRestStorage(storageMap)
	workerRestStorage(storageMap)
	workflowRestStorage(storageMap)
	return storageMap
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

func functionRestStorage(storageMap map[string]rest.Storage) {
	s, err := function.NewREST(makeStoreOptions("function"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["function"] = s
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
	s, err := user.NewREST(makeStoreOptions("stage"))
	if err != nil {
		flog.Panic(err)
	}
	storageMap["stage"] = s
}

func userRestStorage(storageMap map[string]rest.Storage) {
	s, err := user.NewREST(makeStoreOptions("user"))
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
	jsonCoder := runtime.JsonCoder{}
	codec := runtime.NewBase64Serializer(jsonCoder, jsonCoder)
	storeOptions := &options.StoreOptions{
		RESTOptions: &options.RESTOptions{
			StorageConfig: &config.ConfigForResource{
				Config: config.Config{
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
			ResourcePrefix:          fmt.Sprintf("%s/%s/%s", constant.GroupName, constant.Version, resource), //todo
			CountMetricPollPeriod:   0,
		},
	}

	return storeOptions
}
