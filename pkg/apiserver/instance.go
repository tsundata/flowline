package apiserver

import (
	"github.com/tsundata/flowline/pkg/apiserver/config"
	"github.com/tsundata/flowline/pkg/apiserver/controller"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type Instance struct {
	GenericAPIServer   *GenericAPIServer
	AuthenticationInfo *controller.AuthenticationInfo
}

func NewInstance(config *config.Config) *Instance {
	return &Instance{
		GenericAPIServer: NewGenericAPIServer("apiserver", config),
	}
}

func (i *Instance) Run(stopCh <-chan struct{}) error {
	defer i.Destroy()

	err := i.GenericAPIServer.Run(stopCh)
	if err != nil {
		flog.Panic(err)
	}

	<-stopCh

	flog.Info("apiserver is exiting")
	return nil
}

func (i *Instance) Destroy() {

}
