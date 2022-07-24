package variable

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type VariableStorage struct {
	REST *REST
}

func NewStorage(options *options.StoreOptions) (VariableStorage, error) {
	r, err := NewREST(options)
	if err != nil {
		return VariableStorage{}, err
	}
	return VariableStorage{REST: r}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.Variable{} },
		NewListFunc:              func() runtime.Object { return &meta.VariableList{} },
		NewStructFunc:            func() interface{} { return meta.Variable{} },
		NewListStructFunc:        func() interface{} { return meta.VariableList{} },
		DefaultQualifiedResource: rest.Resource("variable"),

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
