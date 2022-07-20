package v1

import (
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/informer"
	"github.com/tsundata/flowline/pkg/informer/informers/internalinterfaces"
	v1 "github.com/tsundata/flowline/pkg/informer/listers/core/v1"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/watch"
	"time"
)

type StageInformer interface {
	Informer() informer.SharedIndexInformer
	Lister() v1.StageLister
}

type stageInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

func newStageInformer(client *client.RestClient, resyncPeriod time.Duration, indexers informer.Indexers) informer.SharedIndexInformer {
	return NewFilteredStageInformer(client, resyncPeriod, indexers, nil)
}

func NewFilteredStageInformer(client *client.RestClient, resyncPeriod time.Duration, indexers informer.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) informer.SharedIndexInformer {
	return informer.NewSharedIndexInformer(
		&informer.ListWatch{
			ListFunc: func(options meta.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return nil, nil //client.Request(context.TODO()).Stage().List().Result()
			},
			WatchFunc: func(options meta.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return nil, nil //client.Request(context.TODO()).Pods().Watch(, options)
			},
		},
		&meta.Stage{},
		resyncPeriod,
		indexers,
	)
}

func (f *stageInformer) defaultInformer(client *client.RestClient, resyncPeriod time.Duration) informer.SharedIndexInformer {
	return NewFilteredStageInformer(client, resyncPeriod, informer.Indexers{}, f.tweakListOptions)
}

func (f *stageInformer) Informer() informer.SharedIndexInformer {
	return f.factory.InformerFor(&meta.Stage{}, f.defaultInformer)
}

func (f *stageInformer) Lister() v1.StageLister {
	return v1.NewStageLister(f.Informer().GetIndexer())
}
