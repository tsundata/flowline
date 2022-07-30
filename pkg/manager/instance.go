package manager

import (
	"github.com/tsundata/flowline/pkg/manager/config"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type Instance struct {
	GenericControllerManagerServer *GenericControllerManagerServer
}

func NewInstance(config *config.Config) *Instance {
	return &Instance{
		GenericControllerManagerServer: NewGenericControllerManagerServer("controller-manager", config),
	}
}

func (i *Instance) Run(stopCh <-chan struct{}) error {
	defer i.Destroy()

	err := i.GenericControllerManagerServer.Run(stopCh)
	if err != nil {
		flog.Panic(err)
	}

	<-stopCh

	flog.Info("controller-manager is exiting")
	return nil
}

func (i *Instance) Destroy() {

}
