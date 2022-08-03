package etcd

import (
	"context"
	"errors"
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/storage"
	"github.com/tsundata/flowline/pkg/apiserver/storage/value"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/watch"
	clientv3 "go.etcd.io/etcd/client/v3"
	"path"
	"reflect"
	"strings"
)

type store struct {
	client        *clientv3.Client
	codec         runtime.Codec
	versioner     storage.Versioner
	transformer   value.Transformer
	pathPrefix    string
	watcher       *watcher
	pagingEnabled bool
	leaseManager  *leaseManager
}

func New(c *clientv3.Client, codec runtime.Codec, newFunc func() runtime.Object, prefix string, transformer value.Transformer, pagingEnabled bool) storage.Interface {
	return newStore(c, codec, newFunc, prefix, transformer, pagingEnabled)
}

func newStore(c *clientv3.Client, codec runtime.Codec, newFunc func() runtime.Object, prefix string, transformer value.Transformer, pagingEnabled bool) *store {
	versioner := storage.APIObjectVersioner{}
	return &store{
		client:        c,
		codec:         codec,
		versioner:     versioner,
		transformer:   transformer,
		pathPrefix:    path.Join("/", prefix),
		pagingEnabled: pagingEnabled,
		watcher:       newWatcher(c, codec, newFunc, versioner, transformer),
		leaseManager:  newDefaultLeaseManager(c, NewDefaultLeaseManagerConfig()),
	}
}

func (s *store) Versioner() storage.Versioner {
	return s.versioner
}

func (s *store) Create(ctx context.Context, key string, obj, out runtime.Object, ttl uint64) error {
	if version, err := s.versioner.ObjectResourceVersion(obj); err == nil && version != 0 {
		return errors.New("resourceVersion should not be set on objects to be created")
	}
	if err := s.versioner.PrepareObjectForStorage(obj); err != nil {
		return fmt.Errorf("PrepareObjectForStorage failed: %v", err)
	}
	data, err := runtime.Encode(s.codec, obj)
	if err != nil {
		return err
	}
	key = path.Join(s.pathPrefix, key)

	opts, err := s.ttlOpts(ctx, int64(ttl))
	if err != nil {
		return err
	}

	newData := data

	txnResp, err := s.client.KV.Txn(ctx).If(
		notFound(key),
	).Then(
		clientv3.OpPut(key, string(newData), opts...),
	).Commit()
	if err != nil {
		return err
	}

	if !txnResp.Succeeded {
		return fmt.Errorf("key exists %s", key)
	}

	if out != nil {
		putResp := txnResp.Responses[0].GetResponsePut()
		err = decode(s.codec, s.versioner, data, out, putResp.Header.Revision)
		return err
	}
	return nil
}

func (s *store) Delete(ctx context.Context, key string, out runtime.Object, preconditions interface{}, validateDeletion interface{}, cachedExistingObject runtime.Object) error {
	key = path.Join(s.pathPrefix, key)
	_, err := s.client.Delete(ctx, key) // todo
	return err
}

func (s *store) Watch(ctx context.Context, key string, opts meta.ListOptions) (watch.Interface, error) {
	rev, err := s.versioner.ParseResourceVersion(opts.ResourceVersion)
	if err != nil {
		return nil, err
	}
	key = path.Join(s.pathPrefix, key)
	return s.watcher.Watch(ctx, key, int64(rev), opts.Recursive, opts.ProgressNotify, opts.Predicate)
}

func (s *store) Get(ctx context.Context, key string, opts meta.GetOptions, out runtime.Object) error {
	key = path.Join(s.pathPrefix, key)
	getResp, err := s.client.KV.Get(ctx, key)
	if err != nil {
		return err
	}
	if len(getResp.Kvs) == 0 {
		if opts.IgnoreNotFound {
			return meta.SetZeroValue(out)
		}
		return fmt.Errorf("key not found %s", key)
	}
	kv := getResp.Kvs[0]
	data := kv.Value

	return decode(s.codec, s.versioner, data, out, kv.ModRevision)
}

func (s *store) GetList(ctx context.Context, key string, opts meta.ListOptions, listObj runtime.Object) error {
	recursive := opts.Recursive
	pred := opts.Predicate
	listPtr, err := meta.GetItemsPtr(listObj)
	if err != nil {
		return err
	}
	v, err := meta.EnforcePtr(listPtr)
	if err != nil || v.Kind() != reflect.Slice {
		return fmt.Errorf("need ptr to slice: %v", err)
	}
	key = path.Join(s.pathPrefix, key)

	// For recursive lists, we need to make sure the key ended with "/" so that we only
	// get children "directories". e.g. if we have key "/a", "/a/b", "/ab", getting keys
	// with prefix "/a" will return all three, while with prefix "/a/" will return only
	// "/a/b" which is the correct answer.
	if recursive && !strings.HasSuffix(key, "/") {
		key += "/"
	}

	newItemFunc := getNewItemFunc(listObj, v)

	var returnedRV int64
	options := make([]clientv3.OpOption, 0, 4)
	options = append(options, clientv3.WithPrefix())
	getResp, err := s.client.KV.Get(ctx, key, options...)
	if err != nil {
		return err
	}
	for _, kv := range getResp.Kvs {
		data := kv.Value
		if err := appendListItem(v, data, uint64(kv.ModRevision), pred, s.codec, s.versioner, newItemFunc); err != nil {
			return err
		}
	}
	// indicate to the client which resource version was returned
	if returnedRV == 0 {
		returnedRV = getResp.Header.Revision
	}

	return s.versioner.UpdateList(listObj, uint64(returnedRV), "", nil)
}

func (s *store) GuaranteedUpdate(ctx context.Context, key string, destination runtime.Object, ignoreNotFound bool, preconditions interface{}, tryUpdate interface{}, cachedExistingObject runtime.Object) error {
	key = path.Join(s.pathPrefix, key)

	getResp, err := s.client.KV.Get(ctx, key)
	if err != nil {
		return err
	}
	if getResp.Kvs == nil {
		return fmt.Errorf("not found %s", key)
	}
	rev := getResp.Kvs[0].ModRevision

	data, err := runtime.Encode(s.codec, destination)
	if err != nil {
		return err
	}

	newData := data

	opts, err := s.ttlOpts(ctx, int64(0)) // todo
	if err != nil {
		return err
	}

	txnResp, err := s.client.KV.Txn(ctx).If(
		clientv3.Compare(clientv3.ModRevision(key), "=", rev),
	).Then(
		clientv3.OpPut(key, string(newData), opts...),
	).Else(
		clientv3.OpGet(key),
	).Commit()
	if err != nil {
		return err
	}
	if !txnResp.Succeeded {
		return fmt.Errorf("update error %s", key)
	}
	putResp := txnResp.Responses[0].GetResponsePut()

	err = decode(s.codec, s.versioner, data, destination, putResp.Header.Revision)
	return err
}

func (s *store) Count(key string) (int64, error) {
	key = path.Join(s.pathPrefix, key)

	// We need to make sure the key ended with "/" so that we only get children "directories".
	// e.g. if we have key "/a", "/a/b", "/ab", getting keys with prefix "/a" will return all three,
	// while with prefix "/a/" will return only "/a/b" which is the correct answer.
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}

	getResp, err := s.client.KV.Get(context.Background(), key, clientv3.WithRange(clientv3.GetPrefixRangeEnd(key)))
	if err != nil {
		return 0, err
	}
	return getResp.Count, nil
}

// ttlOpts returns client options based on given ttl.
// ttl: if ttl is non-zero, it will attach the key to a lease with ttl of roughly the same length
func (s *store) ttlOpts(ctx context.Context, ttl int64) ([]clientv3.OpOption, error) {
	if ttl == 0 {
		return nil, nil
	}
	id, err := s.leaseManager.GetLease(ctx, ttl)
	if err != nil {
		return nil, err
	}
	return []clientv3.OpOption{clientv3.WithLease(id)}, nil
}

// decode decodes value of bytes into object. It will also set the object resource version to rev.
// On success, objPtr would be set to the object.
func decode(codec runtime.Codec, versioner storage.Versioner, value []byte, objPtr runtime.Object, rev int64) error {
	_, _, err := codec.Decode(value, nil, objPtr)
	if err != nil {
		return err
	}
	// being unable to set the version does not prevent the object from being extracted
	if err := versioner.UpdateObject(objPtr, uint64(rev)); err != nil {
		flog.Errorf("failed to update object version: %v", err)
	}
	return nil
}

// appendListItem decodes and appends the object (if it passes filter) to v, which must be a slice.
func appendListItem(v reflect.Value, data []byte, rev uint64, _ meta.SelectionPredicate, codec runtime.Codec, versioner storage.Versioner, newItemFunc func() runtime.Object) error {
	obj, _, err := codec.Decode(data, nil, newItemFunc())
	if err != nil {
		return err
	}
	// being unable to set the version does not prevent the object from being extracted
	if err := versioner.UpdateObject(obj, rev); err != nil {
		flog.Errorf("failed to update object version: %v", err)
	}
	v.Set(reflect.Append(v, reflect.ValueOf(obj).Elem())) // todo

	return nil
}

func notFound(key string) clientv3.Cmp {
	return clientv3.Compare(clientv3.ModRevision(key), "=", 0)
}

func getNewItemFunc(listObj runtime.Object, v reflect.Value) func() runtime.Object {
	// Otherwise just instantiate an empty item
	elem := v.Type().Elem()
	return func() runtime.Object {
		return reflect.New(elem).Interface().(runtime.Object)
	}
}
