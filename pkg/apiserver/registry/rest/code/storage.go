package code

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type CodeStorage struct {
	REST *REST
}

func NewStorage(options *options.StoreOptions) (CodeStorage, error) {
	r, err := NewREST(options)
	if err != nil {
		return CodeStorage{}, err
	}
	return CodeStorage{REST: r}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.Code{} },
		NewListFunc:              func() runtime.Object { return &meta.CodeList{} },
		NewStructFunc:            func() interface{} { return meta.Code{} },
		NewListStructFunc:        func() interface{} { return meta.CodeList{} },
		DefaultQualifiedResource: rest.Resource("code"),

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
