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

type JobInformer interface {
	Informer() informer.SharedIndexInformer
	Lister() v1.JobLister
}

type jobInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

func newJobInformer(client client.Interface, resyncPeriod time.Duration, indexers informer.Indexers) informer.SharedIndexInformer {
	return NewFilteredJobInformer(client, resyncPeriod, indexers, nil)
}

func NewFilteredJobInformer(client client.Interface, resyncPeriod time.Duration, indexers informer.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) informer.SharedIndexInformer {
	return informer.NewSharedIndexInformer(
		&informer.ListWatch{
			ListFunc: func(options meta.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1().Job().List(context.Background(), options)
			},
			WatchFunc: func(options meta.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1().Job().Watch(context.Background(), options)
			},
		},
		&meta.Job{},
		resyncPeriod,
		indexers,
	)
}

func (f *jobInformer) defaultInformer(client client.Interface, resyncPeriod time.Duration) informer.SharedIndexInformer {
	return NewFilteredJobInformer(client, resyncPeriod, informer.Indexers{}, f.tweakListOptions)
}

func (f *jobInformer) Informer() informer.SharedIndexInformer {
	return f.factory.InformerFor(&meta.Job{}, f.defaultInformer)
}

func (f *jobInformer) Lister() v1.JobLister {
	return v1.NewJobLister(f.Informer().GetIndexer())
}
