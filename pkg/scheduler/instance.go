package scheduler

import (
	"github.com/tsundata/flowline/pkg/util/flog"
)

type Instance struct {
	GenericSchedulerServer *GenericSchedulerServer
}

func NewInstance(config *Config) *Instance {
	return &Instance{
		GenericSchedulerServer: NewGenericSchedulerServer("scheduler", config),
	}
}

func (i *Instance) Run(stopCh <-chan struct{}) error {
	defer i.Destroy()

	err := i.GenericSchedulerServer.Run(stopCh)
	if err != nil {
		flog.Panic(err)
	}

	<-stopCh

	flog.Info("scheduler is exiting")
	return nil
}

func (i *Instance) Destroy() {

}
