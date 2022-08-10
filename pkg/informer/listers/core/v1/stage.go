package v1

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/informer"
	"golang.org/x/xerrors"
)

type StageLister interface {
	List(selector interface{}) (ret []*meta.Stage, err error)
	Get(name string) (*meta.Stage, error)
}

type stageLister struct {
	indexer informer.Indexer
}

func NewStageLister(indexer informer.Indexer) StageLister {
	return &stageLister{indexer: indexer}
}

func (s *stageLister) List(selector interface{}) (ret []*meta.Stage, err error) {
	err = informer.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*meta.Stage))
	})
	return ret, err
}

func (s *stageLister) Get(name string) (*meta.Stage, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, xerrors.Errorf("NewNotFound %s", name)
	}
	return obj.(*meta.Stage), nil
}
