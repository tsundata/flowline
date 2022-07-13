package scheduler

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type GenericSchedulerServer struct {
	client *client.RestClient
}

func NewGenericSchedulerServer(name string, config *Config) *GenericSchedulerServer {
	flog.Infof("%s starting...", name)
	s := &GenericSchedulerServer{
		client: client.New(config.ApiURL),
	}
	return s
}

func (g *GenericSchedulerServer) Run(stopCh <-chan struct{}) error {
	go func() {
		fmt.Println("scheduler run")
	}()
	select {
	case <-stopCh:
		flog.Info("stop scheduler server")
	}
	return nil
}
