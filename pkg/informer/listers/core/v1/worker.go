package v1

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/informer"
)

type WorkerLister interface {
	List(selector interface{}) (ret []*meta.Worker, err error)
}

type workerLister struct {
	indexer informer.Indexer
}

func NewWorkerLister(indexer informer.Indexer) WorkerLister {
	return &workerLister{indexer: indexer}
}

func (s *workerLister) List(selector interface{}) (ret []*meta.Worker, err error) {
	err = informer.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*meta.Worker))
	})
	return ret, err
}
