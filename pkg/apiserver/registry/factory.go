package registry

import (
	"github.com/tsundata/flowline/pkg/apiserver/storage"
	"github.com/tsundata/flowline/pkg/apiserver/storage/config"
	"github.com/tsundata/flowline/pkg/apiserver/storage/decorator"
	"github.com/tsundata/flowline/pkg/apiserver/storage/etcd"
	"github.com/tsundata/flowline/pkg/runtime"
	clientv3 "go.etcd.io/etcd/client/v3"
	"sync"
	"time"
)

func StorageFactory() decorator.StorageDecorator {
	return func(config *config.ConfigForResource, resourcePrefix string, keyFunc func(obj runtime.Object) (string, error), newFunc func() runtime.Object, newListFunc func() runtime.Object) (storage.Interface, decorator.DestroyFunc, error) {
		cli, err := clientv3.New(clientv3.Config{ // fixme
			Endpoints:   []string{"127.0.0.1:2379"},
			DialTimeout: 5 * time.Second,
			Username:    "",
			Password:    "",
		})
		if err != nil {
			return nil, nil, err
		}

		s := etcd.New(cli, config.Codec, "", false)

		var once sync.Once
		destroyFunc := func() {
			once.Do(func() {
				// todo
			})
		}

		return s, destroyFunc, nil
	}
}
