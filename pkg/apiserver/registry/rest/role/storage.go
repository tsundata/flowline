package role

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/authorizer"
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
		NewStructFunc:            func() interface{} { return meta.Role{} },
		NewListStructFunc:        func() interface{} { return meta.RoleList{} },
		DefaultQualifiedResource: rest.Resource("role"),

		CreateStrategy:      Strategy,
		UpdateStrategy:      Strategy,
		DeleteStrategy:      Strategy,
		ResetFieldsStrategy: Strategy,
	}

	// AfterUpdate
	store.AfterUpdate = func(obj runtime.Object, options *meta.UpdateOptions) {
		enforcer, err := authorizer.NewEnforcer(authorizer.NewAdapter(&store.Storage))
		enforcer.EnableAutoSave(false)
		if err != nil {
			flog.Error(err)
			return
		}
		if role, ok := obj.(*meta.Role); ok {
			_, err = enforcer.RemoveFilteredPolicy(0, role.Name)
			if err != nil {
				flog.Error(err)
				return
			}
			for _, verb := range role.Verbs {
				for _, resource := range role.Resources {
					_, err = enforcer.AddPolicy(role.Name, resource, verb)
					if err != nil {
						flog.Error(err)
						return
					}
				}
			}
			err = enforcer.SavePolicy()
			if err != nil {
				flog.Error(err)
				return
			}
		}
	}

	err := store.CompleteWithOptions(options)
	if err != nil {
		flog.Panic(err)
	}

	return &REST{store}, nil
}
