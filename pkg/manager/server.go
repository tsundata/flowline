package manager

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type GenericControllerManagerServer struct {
	client *client.RestClient
}

func NewGenericControllerManagerServer(name string, config *Config) *GenericControllerManagerServer {
	flog.Infof("%s starting...", name)
	s := &GenericControllerManagerServer{
		client: client.NewRestClient(config.ApiURL),
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
