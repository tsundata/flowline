package user

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type UserStorage struct {
	REST *REST
}

func NewStorage(options *options.StoreOptions) (UserStorage, error) {
	r, err := NewREST(options)
	if err != nil {
		return UserStorage{}, err
	}
	return UserStorage{REST: r}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.User{} },
		NewListFunc:              func() runtime.Object { return &meta.UserList{} },
		NewStructFunc:            func() interface{} { return meta.User{} },
		NewListStructFunc:        func() interface{} { return meta.UserList{} },
		DefaultQualifiedResource: rest.Resource("user"),

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
