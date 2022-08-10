package informer

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"golang.org/x/xerrors"
)

// AppendFunc is used to add a matching item to whatever list the caller is using
type AppendFunc func(interface{})

// ListAll calls appendFn with each value retrieved from store which matches the selector.
func ListAll(store Store, selector interface{}, appendFn AppendFunc) error {
	selectAll := true
	for _, m := range store.List() {
		if selectAll {
			// Avoid computing labels of the objects to speed up common flows
			// of listing all objects.
			appendFn(m)
			continue
		}
		_, err := meta.Accessor(m)
		if err != nil {
			return err
		}
		if selector == nil {
			appendFn(m)
		}
	}
	return nil
}

// GenericLister is a lister skin on a generic Indexer
type GenericLister interface {
	// List will return all objects across namespaces
	List(selector interface{}) (ret []runtime.Object, err error)
	// Get will attempt to retrieve assuming that name==key
	Get(name string) (runtime.Object, error)
}

// NewGenericLister creates a new instance for the genericLister.
func NewGenericLister(indexer Indexer, resource schema.GroupResource) GenericLister {
	return &genericLister{indexer: indexer, resource: resource}
}

type genericLister struct {
	indexer  Indexer
	resource schema.GroupResource
}

func (s *genericLister) List(selector interface{}) (ret []runtime.Object, err error) {
	err = ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(runtime.Object))
	})
	return ret, err
}

func (s *genericLister) Get(name string) (runtime.Object, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, xerrors.Errorf("NewNotFound %s %s", s.resource, name)
	}
	return obj.(runtime.Object), nil
}
