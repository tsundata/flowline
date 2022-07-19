package informer

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/storage/etcd/watch"
	"github.com/tsundata/flowline/pkg/runtime"
	"time"
)

func SharedInformerFactory(client interface{}, exampleObject runtime.Object, resyncPeriod time.Duration, indexers Indexers) SharedIndexInformer {
	return NewSharedIndexInformer(
		&ListWatch{
			ListFunc: func(options meta.ListOptions) (runtime.Object, error) {
				return nil, nil
			},
			WatchFunc: func(options meta.ListOptions) (watch.Interface, error) {
				return nil, nil
			},
		},
		exampleObject,
		resyncPeriod,
		indexers,
	)
}
