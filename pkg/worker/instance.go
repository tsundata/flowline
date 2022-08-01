package worker

import (
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/worker/config"
)

type Instance struct {
	GenericWorkerServer *GenericWorkerServer
}

func NewInstance(config *config.Config) *Instance {
	return &Instance{
		GenericWorkerServer: NewGenericWorkerServer("worker", config),
	}
}

func (i *Instance) Run(stopCh <-chan struct{}) error {
	defer i.Destroy()

	err := i.GenericWorkerServer.Run(stopCh)
	if err != nil {
		flog.Panic(err)
	}

	<-stopCh

	flog.Info("worker is exiting")
	return nil
}

func (i *Instance) Destroy() {

}
