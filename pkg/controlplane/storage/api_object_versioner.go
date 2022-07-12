package storage

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"strconv"
)

// APIObjectVersioner implements versioning and extracting etcd node information
// for objects that have an embedded ObjectMeta or ListMeta field.
// source: staging/src/k8s.io/apiserver/pkg/storage/api_object_versioner.go
type APIObjectVersioner struct{}

// UpdateObject implements Versioner
func (a APIObjectVersioner) UpdateObject(obj interface{}, resourceVersion uint64) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	versionString := ""
	if resourceVersion != 0 {
		versionString = strconv.FormatUint(resourceVersion, 10)
	}
	accessor.SetResourceVersion(versionString)
	return nil
}

// UpdateList implements Versioner
func (a APIObjectVersioner) UpdateList(obj interface{}, resourceVersion uint64, nextKey string, count *int64) error {
	if resourceVersion == 0 {
		return fmt.Errorf("illegal resource version from storage: %d", resourceVersion)
	}
	listAccessor, err := meta.ListAccessor(obj)
	if err != nil || listAccessor == nil {
		return err
	}
	versionString := strconv.FormatUint(resourceVersion, 10)
	listAccessor.SetResourceVersion(versionString)
	listAccessor.SetContinue(nextKey)
	listAccessor.SetRemainingItemCount(count)
	return nil
}

// PrepareObjectForStorage clears resourceVersion and selfLink prior to writing to etcd.
func (a APIObjectVersioner) PrepareObjectForStorage(obj interface{}) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	accessor.SetResourceVersion("")
	return nil
}

// ObjectResourceVersion implements Versioner
func (a APIObjectVersioner) ObjectResourceVersion(obj interface{}) (uint64, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return 0, err
	}
	version := accessor.GetResourceVersion()
	if len(version) == 0 {
		return 0, nil
	}
	return strconv.ParseUint(version, 10, 64)
}

// ParseResourceVersion takes a resource version argument and converts it to
// the etcd version. For watch we should pass to helper.Watch(). Because resourceVersion is
// an opaque value, the default watch behavior for non-zero watch is to watch
// the next value (if you pass "1", you will see updates from "2" onwards).
func (a APIObjectVersioner) ParseResourceVersion(resourceVersion string) (uint64, error) {
	if resourceVersion == "" || resourceVersion == "0" {
		return 0, nil
	}
	version, err := strconv.ParseUint(resourceVersion, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s, %s",
			// Validation errors are supposed to return version-specific field
			// paths, but this is probably close enough.
			resourceVersion, err.Error(),
		)
	}
	return version, nil
}

// Versioner implements Versioner
var _ Versioner = APIObjectVersioner{}

// CompareResourceVersion compares etcd resource versions.  Outside this API they are all strings,
// but etcd resource versions are special, they're actually ints, so we can easily compare them.
func (a APIObjectVersioner) CompareResourceVersion(lhs, rhs interface{}) int {
	lhsVersion, err := a.ObjectResourceVersion(lhs)
	if err != nil {
		// coder error
		panic(err)
	}
	rhsVersion, err := a.ObjectResourceVersion(rhs)
	if err != nil {
		// coder error
		panic(err)
	}

	if lhsVersion == rhsVersion {
		return 0
	}
	if lhsVersion < rhsVersion {
		return -1
	}

	return 1
}
