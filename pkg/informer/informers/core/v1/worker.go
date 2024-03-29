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

type WorkerInformer interface {
	Informer() informer.SharedIndexInformer
	Lister() v1.WorkerLister
}

type workerInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

func NewFilteredWorkerInformer(client client.Interface, resyncPeriod time.Duration, indexers informer.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) informer.SharedIndexInformer {
	return informer.NewSharedIndexInformer(
		&informer.ListWatch{
			ListFunc: func(options meta.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1().Worker().List(context.Background(), options)
			},
			WatchFunc: func(options meta.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1().Worker().Watch(context.Background(), options)
			},
		},
		&meta.Worker{},
		resyncPeriod,
		indexers,
	)
}

func (f *workerInformer) defaultInformer(client client.Interface, resyncPeriod time.Duration) informer.SharedIndexInformer {
	return NewFilteredWorkerInformer(client, resyncPeriod, informer.Indexers{}, f.tweakListOptions)
}

func (f *workerInformer) Informer() informer.SharedIndexInformer {
	return f.factory.InformerFor(&meta.Worker{}, f.defaultInformer)
}

func (f *workerInformer) Lister() v1.WorkerLister {
	return v1.NewWorkerLister(f.Informer().GetIndexer())
}
