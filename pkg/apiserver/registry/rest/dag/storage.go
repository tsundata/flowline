package dag

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type DagStorage struct {
	REST *REST
}

func NewStorage(options *options.StoreOptions) (DagStorage, error) {
	r, err := NewREST(options)
	if err != nil {
		return DagStorage{}, err
	}
	return DagStorage{REST: r}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.Dag{} },
		NewListFunc:              func() runtime.Object { return &meta.DagList{} },
		NewStructFunc:            func() interface{} { return meta.Dag{} },
		NewListStructFunc:        func() interface{} { return meta.DagList{} },
		DefaultQualifiedResource: rest.Resource("dag"),

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
