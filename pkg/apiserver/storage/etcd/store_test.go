package etcd

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime/serializer/json"
	clientv3 "go.etcd.io/etcd/client/v3"
	"testing"
	"time"
)

func TestEtcdStore(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
		Username:    "",
		Password:    "",
	})
	if err != nil {
		t.Fatal(err)
	}

	obj := &meta.Workflow{Trigger: meta.TriggerCron}
	ctx := context.Background()
	coder := json.NewSerializerWithOptions(json.DefaultMetaFactory, json.SerializerOptions{})
	s := New(cli, coder, nil, "", nil, false)
	err = s.Create(ctx, "test"+time.Now().String(), obj, obj, 10000)
	if err != nil {
		t.Fatal(err)
	}
}
