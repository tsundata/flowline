package meta

import (
	"github.com/tsundata/flowline/pkg/controlplane/runtime"
	"github.com/tsundata/flowline/pkg/controlplane/runtime/schema"
)

type Dag struct {
	TypeMeta
	ObjectMeta
}

func (d *Dag) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (d *Dag) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}

type DagList struct {
	TypeMeta
	ObjectMeta
	Items []Dag
}

func (d *DagList) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (d *DagList) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}
