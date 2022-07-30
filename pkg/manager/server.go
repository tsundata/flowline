package manager

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/manager/config"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type GenericControllerManagerServer struct {
	client client.Interface
}

func NewGenericControllerManagerServer(name string, config *config.Config) *GenericControllerManagerServer {
	flog.Infof("%s starting...", name)
	c, err := client.NewForConfig(config.RestConfig)
	if err != nil {
		panic(err)
	}
	s := &GenericControllerManagerServer{
		client: c,
	}
	return s
}

func (g *GenericControllerManagerServer) Run(stopCh <-chan struct{}) error {
	go func() {
		fmt.Println("controller-manager run")
	}()
	select {
	case <-stopCh:
		flog.Info("stop controller-manager server")
	}
	return nil
}
