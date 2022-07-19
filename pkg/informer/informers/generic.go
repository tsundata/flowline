package informers

import (
	"github.com/tsundata/flowline/pkg/informer"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type GenericInformer interface {
	Informer() informer.SharedIndexInformer
	Lister() informer.GenericLister
}

type genericInformer struct {
	informer informer.SharedIndexInformer
	resource schema.GroupResource
}

// Informer returns the SharedIndexInformer.
func (f *genericInformer) Informer() informer.SharedIndexInformer {
	return f.informer
}

// Lister returns the GenericLister.
func (f *genericInformer) Lister() informer.GenericLister {
	return informer.NewGenericLister(f.Informer().GetIndexer(), f.resource)
}
