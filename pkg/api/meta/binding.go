package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type Binding struct {
	TypeMeta
	ObjectMeta

	Target Worker `json:"target,omitempty"`
}

func (m *Binding) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Binding) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}
