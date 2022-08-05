package meta

import (
	"errors"
	"fmt"
	"github.com/tsundata/flowline/pkg/runtime"
	"reflect"
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

// Common lets you work with core metadata from any of the versioned or
// internal API objects. Attempting to set or retrieve a field on an object that does
// not support that field will be a no-op and return a default value.
type Common interface {
	GetResourceVersion() string
	SetResourceVersion(version string)
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

func ListAccessor(obj interface{}) (List, error) {
	switch t := obj.(type) {
	case List:
		return t, nil
	case ListMetaAccessor:
		if m := t.GetListMeta(); m != nil {
			return m, nil
		}
		return nil, errors.New("object does not implement the List interfaces")
	default:
		return nil, errors.New("object does not implement the List interfaces")
	}
}

func SetZeroValue(objPtr runtime.Object) error {
	v, err := runtime.EnforcePtr(objPtr)
	if err != nil {
		return err
	}
	v.Set(reflect.Zero(v.Type()))
	return nil
}

// errNotList is returned when an object implements the Object style interfaces but not the List style
// interfaces.
// var errNotList = fmt.Errorf("object does not implement the List interfaces")

var errNotCommon = fmt.Errorf("object does not implement the common interface for accessing the SelfLink")

// CommonAccessor returns a Common interface for the provided object or an error if the object does
// not provide List.
func CommonAccessor(obj interface{}) (Common, error) {
	switch t := obj.(type) {
	case List:
		return t, nil
	case ListMetaAccessor:
		if m := t.GetListMeta(); m != nil {
			return m, nil
		}
		return nil, errNotCommon
	case Object:
		return t, nil
	case ObjectMetaAccessor:
		if m := t.GetObjectMeta(); m != nil {
			return m, nil
		}
		return nil, errNotCommon
	default:
		return nil, errNotCommon
	}
}

// ExtractList returns obj's Items element as an array of runtime.Objects.
// Returns an error if obj is not a List type (does not have an Items member).
func ExtractList(obj runtime.Object) ([]runtime.Object, error) {
	itemsPtr, err := runtime.GetItemsPtr(obj)
	if err != nil {
		return nil, err
	}
	items, err := runtime.EnforcePtr(itemsPtr)
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

func FactoryNewObject(kind string) runtime.Object {
	switch kind {
	case "workflow":
		return &Workflow{}
	case "job":
		return &Job{}
	case "stage":
		return &Stage{}
	case "worker":
		return &Worker{}
	default:
		return &Unknown{}
	}
}
