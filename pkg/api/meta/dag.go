package meta

import (
	dagLib "github.com/heimdalr/dag"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"golang.org/x/xerrors"
)

type Dag struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:",inline"`

	WorkflowUID string `json:"workflowUID,omitempty"`
	Nodes       []Node `json:"nodes"`
	Edges       []Edge `json:"edges"`
}

type NodeStatus string

const (
	NodeDefault    NodeStatus = "default"
	NodeSuccess    NodeStatus = "success"
	NodeProcessing NodeStatus = "processing"
	NodeError      NodeStatus = "error"
	NodeWarning    NodeStatus = "warning"
)

type Node struct {
	Id        string `json:"id,omitempty"`
	X         int    `json:"x,omitempty"`
	Y         int    `json:"y,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Label     string `json:"label,omitempty"`
	RenderKey string `json:"renderKey,omitempty"`
	IsGroup   bool   `json:"isGroup,omitempty"`
	Group     string `json:"group,omitempty"`
	ParentId  string `json:"parentId,omitempty"`
	Ports     []struct {
		Id        string `json:"id,omitempty"`
		Group     string `json:"group,omitempty"`
		Type      string `json:"type,omitempty"`
		Tooltip   string `json:"tooltip,omitempty"`
		Connected bool   `json:"connected,omitempty"`
	} `json:"ports,omitempty"`
	Order       int        `json:"_order,omitempty"`
	Code        string     `json:"code"`
	Variables   []string   `json:"variables"`
	Connections []string   `json:"connections"`
	Status      NodeStatus `json:"status,omitempty"`
}

type Edge struct {
	Id                string `json:"id,omitempty"`
	Source            string `json:"source,omitempty"`
	Target            string `json:"target,omitempty"`
	SourcePortId      string `json:"sourcePortId,omitempty"`
	TargetPortId      string `json:"targetPortId,omitempty"`
	Label             string `json:"label,omitempty"`
	EdgeContentWidth  int    `json:"edgeContentWidth,omitempty"`
	EdgeContentHeight int    `json:"edgeContentHeight,omitempty"`
	Connector         struct {
		Name string `json:"name,omitempty"`
	} `json:"connector"`
	Router struct {
		Name string `json:"name,omitempty"`
	} `json:"router"`
	SourcePort string `json:"sourcePort,omitempty"`
	TargetPort string `json:"targetPort,omitempty"`
}

func (m *Dag) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Dag) DeepCopyObject() runtime.Object {
	return m
}

type nodeId string

func (n nodeId) ID() string {
	return string(n)
}

func (m *Dag) Validate() error {
	d := dagLib.NewDAG()
	for _, node := range m.Nodes {
		if node.Code == "" {
			return xerrors.Errorf("dag %s node %s not code error", m.UID, node.Id)
		}
		_, err := d.AddVertex(nodeId(node.Id))
		if err != nil {
			return err
		}
	}
	for _, edge := range m.Edges {
		err := d.AddEdge(edge.Source, edge.Target)
		if err != nil {
			return err
		}
	}
	return nil
}

type DagList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:",inline"`

	Items []Dag `json:"items"`
}

func (m *DagList) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *DagList) DeepCopyObject() runtime.Object {
	return m
}
