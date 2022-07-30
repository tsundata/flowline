package v1

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/informer"
	"github.com/tsundata/flowline/pkg/informer/informers/internalinterfaces"
	v1 "github.com/tsundata/flowline/pkg/informer/listers/core/v1"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/watch"
	"time"
)

type WorkflowInformer interface {
	Informer() informer.SharedIndexInformer
	Lister() v1.WorkflowLister
}

type workflowInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

func newWorkflowInformer(client client.Interface, resyncPeriod time.Duration, indexers informer.Indexers) informer.SharedIndexInformer {
	return NewFilteredWorkflowInformer(client, resyncPeriod, indexers, nil)
}

func NewFilteredWorkflowInformer(client client.Interface, resyncPeriod time.Duration, indexers informer.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) informer.SharedIndexInformer {
	return informer.NewSharedIndexInformer(
		&informer.ListWatch{
			ListFunc: func(options meta.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1().Workflow().List(context.Background(), options)
			},
			WatchFunc: func(options meta.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1().Workflow().Watch(context.Background(), options)
			},
		},
		&meta.Workflow{},
		resyncPeriod,
		indexers,
	)
}

func (f *workflowInformer) defaultInformer(client client.Interface, resyncPeriod time.Duration) informer.SharedIndexInformer {
	return NewFilteredWorkflowInformer(client, resyncPeriod, informer.Indexers{}, f.tweakListOptions)
}

func (f *workflowInformer) Informer() informer.SharedIndexInformer {
	return f.factory.InformerFor(&meta.Workflow{}, f.defaultInformer)
}

func (f *workflowInformer) Lister() v1.WorkflowLister {
	return v1.NewWorkflowLister(f.Informer().GetIndexer())
}
