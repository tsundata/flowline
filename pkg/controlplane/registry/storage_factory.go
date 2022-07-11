package registry

import (
	"github.com/tsundata/flowline/pkg/controlplane/registry/generic"
	"github.com/tsundata/flowline/pkg/controlplane/runtime"
	"github.com/tsundata/flowline/pkg/controlplane/storage"
	"github.com/tsundata/flowline/pkg/controlplane/storage/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"
	"sync"
	"time"
)

func StorageFactory() generic.StorageDecorator {
	return func(config *storage.ConfigForResource, resourcePrefix string, keyFunc func(obj runtime.Object) (string, error), newFunc func() runtime.Object, newListFunc func() runtime.Object) (storage.Interface, generic.DestroyFunc, error) {
		cli, err := clientv3.New(clientv3.Config{ // fixme
			Endpoints:   []string{"127.0.0.1:2379"},
			DialTimeout: 5 * time.Second,
			Username:    "",
			Password:    "",
		})
		if err != nil {
			return nil, nil, err
		}

		s := etcd.New(cli, config.Codec, resourcePrefix, false)

		var once sync.Once
		destroyFunc := func() {
			once.Do(func() {
				// todo
			})
		}

		return s, destroyFunc, nil
	}
}
