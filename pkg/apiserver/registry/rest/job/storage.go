package job

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/options"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type JobStorage struct {
	REST *REST
}

func NewStorage(options *options.StoreOptions) (JobStorage, error) {
	r, err := NewREST(options)
	if err != nil {
		return JobStorage{}, err
	}
	return JobStorage{REST: r}, nil
}

type REST struct {
	*registry.Store
}

func NewREST(options *options.StoreOptions) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &meta.Job{} },
		NewListFunc:              func() runtime.Object { return &meta.JobList{} },
		NewStructFunc:            func() interface{} { return meta.Job{} },
		NewListStructFunc:        func() interface{} { return meta.JobList{} },
		DefaultQualifiedResource: rest.Resource("job"),

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
