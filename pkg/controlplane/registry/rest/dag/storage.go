package dag

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/controlplane/registry"
	"github.com/tsundata/flowline/pkg/controlplane/registry/options"
	"github.com/tsundata/flowline/pkg/controlplane/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
)

type DagStorage struct {
	Dag *REST
}

func NewStorage(options *options.StoreOptions) (DagStorage, error) {
	dagRest, err := NewREST(options)
	if err != nil {
		return DagStorage{}, err
	}
	return DagStorage{Dag: dagRest}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.Dag{} },
		NewListFunc:              func() runtime.Object { return &meta.DagList{} },
		DefaultQualifiedResource: rest.Resource("dag"),

		CreateStrategy:      Strategy,
		UpdateStrategy:      Strategy,
		DeleteStrategy:      Strategy,
		ResetFieldsStrategy: Strategy,
	}

	err := store.CompleteWithOptions(options)
	if err != nil {
		panic(err)
	}

	return &REST{store}, nil
}
