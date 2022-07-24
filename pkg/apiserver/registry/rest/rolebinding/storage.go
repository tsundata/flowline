package rolebinding

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type RoleBindingStorage struct {
	REST *REST
}

func NewStorage(options *options.StoreOptions) (RoleBindingStorage, error) {
	r, err := NewREST(options)
	if err != nil {
		return RoleBindingStorage{}, err
	}
	return RoleBindingStorage{REST: r}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.RoleBinding{} },
		NewListFunc:              func() runtime.Object { return &meta.RoleBindingList{} },
		NewStructFunc:            func() interface{} { return meta.RoleBinding{} },
		NewListStructFunc:        func() interface{} { return meta.RoleBindingList{} },
		DefaultQualifiedResource: rest.Resource("rolebinding"),

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
