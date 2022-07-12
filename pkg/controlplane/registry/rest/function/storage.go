package function

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/controlplane/registry"
	"github.com/tsundata/flowline/pkg/controlplane/registry/options"
	"github.com/tsundata/flowline/pkg/controlplane/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type FunctionStorage struct {
	REST *REST
}

func NewStorage(options *options.StoreOptions) (FunctionStorage, error) {
	r, err := NewREST(options)
	if err != nil {
		return FunctionStorage{}, err
	}
	return FunctionStorage{REST: r}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.Function{} },
		NewListFunc:              func() runtime.Object { return &meta.FunctionList{} },
		DefaultQualifiedResource: rest.Resource("function"),

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
