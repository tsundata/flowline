package worker

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type WorkerStorage struct {
	REST *REST
}

func NewStorage(options *options.StoreOptions) (WorkerStorage, error) {
	r, err := NewREST(options)
	if err != nil {
		return WorkerStorage{}, err
	}
	return WorkerStorage{REST: r}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.Worker{} },
		NewListFunc:              func() runtime.Object { return &meta.WorkerList{} },
		NewStructFunc:            func() interface{} { return meta.Worker{} },
		NewListStructFunc:        func() interface{} { return meta.WorkerList{} },
		DefaultQualifiedResource: rest.Resource("worker"),

		CreateStrategy:      Strategy,
		UpdateStrategy:      Strategy,
		DeleteStrategy:      Strategy,
		ResetFieldsStrategy: Strategy,
	}

	err := store.CompleteWithOptions(options)
	if err != nil {
		flog.Panic(err)
	}

	return &REST{store}, nil
}
