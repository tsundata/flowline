package v1

import (
	"github.com/tsundata/flowline/pkg/informer/informers/internalinterfaces"
)

type Interface interface {
	Workers() WorkerInformer
	Stages() StageInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

func New(f internalinterfaces.SharedInformerFactory, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, tweakListOptions: tweakListOptions}
}

func (v *version) Workers() WorkerInformer {
	return &workerInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

func (v *version) Stages() StageInformer {
	return &stageInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}
