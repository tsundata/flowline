package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type Dag struct {
	TypeMeta
	ObjectMeta

	WorkflowUID string
	Nodes       []Node
	Edges       []Edge
}

type Node struct {
	Id        string `json:"id,omitempty"`
	RenderKey string `json:"render_key,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Label     string `json:"label,omitempty"`
	Ports     []struct {
		Id      string `json:"id,omitempty"`
		Type    string `json:"type,omitempty"`
		Group   string `json:"group,omitempty"`
		Tooltip string `json:"tooltip,omitempty"`
	} `json:"ports,omitempty"`
	X     int    `json:"x,omitempty"`
	Y     int    `json:"y,omitempty"`
	Order string `json:"_order,omitempty"`
}

type Edge struct {
	Id           string `json:"id,omitempty"`
	Source       string `json:"source,omitempty"`
	Target       string `json:"target,omitempty"`
	SourcePortId string `json:"source_port_id,omitempty"`
	TargetPortId string `json:"target_port_id,omitempty"`
	Connector    struct {
		Name string `json:"name,omitempty"`
	} `json:"connector"`
	Router struct {
		Name string `json:"name,omitempty"`
	} `json:"router"`
	SourcePort string `json:"source_port,omitempty"`
	TargetPort string `json:"target_port,omitempty"`
}

func (m *Dag) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (m *Dag) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}

type DagList struct {
	TypeMeta
	ObjectMeta
	Items []Dag
}

func (m *DagList) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (m *DagList) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}
