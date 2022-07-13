package role

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type RoleStorage struct {
	REST *REST
}

func NewStorage(options *options.StoreOptions) (RoleStorage, error) {
	r, err := NewREST(options)
	if err != nil {
		return RoleStorage{}, err
	}
	return RoleStorage{REST: r}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.Role{} },
		NewListFunc:              func() runtime.Object { return &meta.RoleList{} },
		DefaultQualifiedResource: rest.Resource("role"),

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
