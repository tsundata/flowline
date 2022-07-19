package meta

import (
	"errors"
	"fmt"
	"github.com/tsundata/flowline/pkg/runtime"
	"time"
)

type ObjectMetaAccessor interface {
	GetObjectMeta() Object
}

// Object lets you work with object metadata from any of the versioned or
// internal API objects. Attempting to set or retrieve a field on an object that does
// not support that field (Name, UID, Namespace on lists) will be a no-op and return
// a default value.
type Object interface {
	GetName() string
	SetName(name string)
	GetUID() string
	SetUID(uid string)
	GetResourceVersion() string
	SetResourceVersion(version string)
	GetGeneration() int64
	SetGeneration(generation int64)
	GetCreationTimestamp() *time.Time
	SetCreationTimestamp(timestamp *time.Time)
	GetDeletionTimestamp() *time.Time
	SetDeletionTimestamp(timestamp *time.Time)
	GetDeletionGracePeriodSeconds() *int64
	SetDeletionGracePeriodSeconds(*int64)
	GetLabels() map[string]string
	SetLabels(labels map[string]string)
}

func Accessor(obj interface{}) (Object, error) {
	switch t := obj.(type) {
	case Object:
		return t, nil
	case ObjectMetaAccessor:
		if m := t.GetObjectMeta(); m != nil {
			return m, nil
		}
		return nil, errors.New("error object type")
	default:
		return nil, errors.New("error object type")
	}
}

type List ListInterface

type ListMetaAccessor interface {
	GetListMeta() List
}

// ListInterface lets you work with list metadata from any of the versioned or
// internal API objects. Attempting to set or retrieve a field on an object that does
// not support that field will be a no-op and return a default value.
type ListInterface interface {
	GetResourceVersion() string
	SetResourceVersion(version string)
	GetContinue() string
	SetContinue(c string)
	GetRemainingItemCount() *int64
	SetRemainingItemCount(c *int64)
}

func ListAccessor(obj interface{}) (List, error) { //fixme
	switch t := obj.(type) {
	case List:
		return t, nil
	case ListMetaAccessor:
		if m := t.GetListMeta(); m != nil {
			return m, nil
		}
		return nil, errors.New("error list type")
	default:
		return nil, errors.New("error list type")
	}
}

func SetZeroValue(obj interface{}) error { //fixme
	return nil
}

// ExtractList returns obj's Items element as an array of runtime.Objects.
// Returns an error if obj is not a List type (does not have an Items member).
func ExtractList(obj runtime.Object) ([]runtime.Object, error) {
	itemsPtr, err := GetItemsPtr(obj)
	if err != nil {
		return nil, err
	}
	items, err := EnforcePtr(itemsPtr)
	if err != nil {
		return nil, err
	}
	list := make([]runtime.Object, items.Len())
	for i := range list {
		raw := items.Index(i)
		switch item := raw.Interface().(type) {
		case RawExtension:
			switch {
			case item.Object != nil:
				list[i] = item.Object
			case item.Raw != nil:
				list[i] = &Unknown{Raw: item.Raw}
			default:
				list[i] = nil
			}
		case runtime.Object:
			list[i] = item
		default:
			var found bool
			if list[i], found = raw.Addr().Interface().(runtime.Object); !found {
				return nil, fmt.Errorf("%v: item[%v]: Expected object, got %#v(%s)", obj, i, raw.Interface(), raw.Kind())
			}
		}
	}
	return list, nil
}
