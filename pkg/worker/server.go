package worker

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type GenericWorkerServer struct {
	client *client.RestClient
}

func NewGenericWorkerServer(name string, config *Config) *GenericWorkerServer {
	flog.Infof("%s starting...", name)
	s := &GenericWorkerServer{
		client: client.New(config.ApiURL),
	}
	return s
}

func (g *GenericWorkerServer) Run(stopCh <-chan struct{}) error {
	go func() {
		fmt.Println("worker run")
	}()
	select {
	case <-stopCh:
		flog.Info("stop worker server")
	}
	return nil
}
