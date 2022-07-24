package connection

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type ConnectionStorage struct {
	REST *REST
}

func NewStorage(options *options.StoreOptions) (ConnectionStorage, error) {
	r, err := NewREST(options)
	if err != nil {
		return ConnectionStorage{}, err
	}
	return ConnectionStorage{REST: r}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.Connection{} },
		NewListFunc:              func() runtime.Object { return &meta.ConnectionList{} },
		NewStructFunc:            func() interface{} { return meta.Connection{} },
		NewListStructFunc:        func() interface{} { return meta.ConnectionList{} },
		DefaultQualifiedResource: rest.Resource("connection"),

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
