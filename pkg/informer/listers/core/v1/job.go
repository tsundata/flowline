package v1

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/informer"
)

type JobLister interface {
	List(selector interface{}) (ret []*meta.Job, err error)
	Get(name string) (*meta.Job, error)
}

type jobLister struct {
	indexer informer.Indexer
}

func NewJobLister(indexer informer.Indexer) JobLister {
	return &jobLister{indexer: indexer}
}

func (s *jobLister) List(selector interface{}) (ret []*meta.Job, err error) {
	err = informer.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*meta.Job))
	})
	return ret, err
}

func (s *jobLister) Get(name string) (*meta.Job, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, informer.ErrNotFound
	}

	return obj.(*meta.Job), nil
}
