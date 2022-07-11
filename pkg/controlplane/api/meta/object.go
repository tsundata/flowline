package meta

import (
	"errors"
	"github.com/tsundata/flowline/pkg/controlplane/runtime"
	"github.com/tsundata/flowline/pkg/controlplane/runtime/schema"
	"time"
)

type Object struct {
	Name               string            `json:"name,omitempty"`
	UID                string            `json:"uid,omitempty"`
	ResourceVersion    string            `json:"resource_version,omitempty"`
	Continue           string            `json:"continue,omitempty"`
	RemainingItemCount *int64            `json:"remaining_item_count,omitempty"`
	SelfLink           string            `json:"self_link,omitempty"`
	CreationTimestamp  time.Time         `json:"creation_timestamp,omitempty"`
	DeletionTimestamp  time.Time         `json:"deletion_timestamp,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
}

func (o *Object) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (o *Object) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}

func Accessor(obj interface{}) (*Object, error) {
	switch t := obj.(type) {
	case *Object:
		return t, nil
	default:
		return nil, errors.New("error object type")
	}
}

func ListAccessor(obj interface{}) ([]*Object, error) {
	switch t := obj.(type) {
	case []*Object:
		return t, nil
	default:
		return nil, errors.New("error object type")
	}
}

func SetZeroValue(obj interface{}) error {
	return nil
}
