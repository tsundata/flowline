package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type Stage struct {
	TypeMeta
	ObjectMeta

	JobUID string
	DagUID string

	State string // todo

	Runtime string
	Code    string

	Input  interface{}
	Output interface{}

	Connection []Connection
	Variable   []Variable
}

func (m *Stage) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (m *Stage) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}
