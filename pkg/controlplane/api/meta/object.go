package meta

import "errors"

type Object struct {
	Value              string `json:"value,omitempty"`
	ResourceVersion    string `json:"resource_version,omitempty"`
	Continue           string `json:"continue,omitempty"`
	RemainingItemCount *int64 `json:"remaining_item_count,omitempty"`
	SelfLink           string `json:"self_link,omitempty"`
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
