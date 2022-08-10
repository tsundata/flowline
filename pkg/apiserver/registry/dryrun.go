package registry

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/storage"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/watch"
	"golang.org/x/xerrors"
)

type DryRunnableStorage struct {
	Storage storage.Interface
	Codec   runtime.Codec
}

func (s *DryRunnableStorage) Versioner() storage.Versioner {
	return s.Storage.Versioner()
}

func (s *DryRunnableStorage) Create(ctx context.Context, key string, obj, out runtime.Object, ttl uint64, dryRun bool) error {
	if dryRun {
		if err := s.Storage.Get(ctx, key, meta.GetOptions{}, out); err == nil {
			return xerrors.Errorf("KeyExistsError %s", key)
		}
		return s.copyInto(obj, out)
	}
	return s.Storage.Create(ctx, key, obj, out, ttl)
}

func (s *DryRunnableStorage) Delete(ctx context.Context, key string, out runtime.Object, preconditions interface{}, deleteValidation interface{}, _ bool, cachedExistingObject runtime.Object) error {
	return s.Storage.Delete(ctx, key, out, preconditions, deleteValidation, cachedExistingObject)
}

func (s *DryRunnableStorage) Watch(ctx context.Context, key string, opts meta.ListOptions) (watch.Interface, error) {
	return s.Storage.Watch(ctx, key, opts)
}

func (s *DryRunnableStorage) Get(ctx context.Context, key string, opts meta.GetOptions, objPtr runtime.Object) error {
	return s.Storage.Get(ctx, key, opts, objPtr)
}

func (s *DryRunnableStorage) GetList(ctx context.Context, key string, opts meta.ListOptions, listObj runtime.Object) error {
	return s.Storage.GetList(ctx, key, opts, listObj)
}

func (s *DryRunnableStorage) GuaranteedUpdate(
	ctx context.Context, key string, destination runtime.Object, ignoreNotFound bool,
	preconditions interface{}, tryUpdate interface{}, _ bool, cachedExistingObject runtime.Object) error {
	return s.Storage.GuaranteedUpdate(ctx, key, destination, ignoreNotFound, preconditions, tryUpdate, cachedExistingObject)
}

func (s *DryRunnableStorage) Count(key string) (int64, error) {
	return s.Storage.Count(key)
}

func (s *DryRunnableStorage) copyInto(in, out runtime.Object) error {
	var data []byte

	data, err := runtime.Encode(s.Codec, in)
	if err != nil {
		return err
	}
	_, _, err = s.Codec.Decode(data, nil, out)
	if err != nil {
		return err
	}
	return nil
}
