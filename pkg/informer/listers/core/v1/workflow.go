package v1

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/informer"
)

type WorkflowLister interface {
	List(selector interface{}) (ret []*meta.Workflow, err error)
}

type workflowLister struct {
	indexer informer.Indexer
}

func NewWorkflowLister(indexer informer.Indexer) WorkflowLister {
	return &workflowLister{indexer: indexer}
}

func (s *workflowLister) List(selector interface{}) (ret []*meta.Workflow, err error) {
	err = informer.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*meta.Workflow))
	})
	return ret, err
}
