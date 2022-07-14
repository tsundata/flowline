package worker

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/worker/stage"
	"net"
)

type GenericWorkerServer struct {
	config *Config
	client *client.RestClient
}

func NewGenericWorkerServer(name string, config *Config) *GenericWorkerServer {
	flog.Infof("%s starting...", name)
	s := &GenericWorkerServer{
		config: config,
		client: client.NewRestClient(config.ApiURL),
	}
	return s
}

func (g *GenericWorkerServer) Run(stopCh <-chan struct{}) error {
	// handle stage
	go func() {
		addr := fmt.Sprintf("%s:%d", g.config.Host, g.config.Port)
		flog.Infof("worker %s starting", addr)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			flog.Fatal(err)
		}
		defer listener.Close()

		for {
			conn, err := listener.Accept()
			if err != nil {
				flog.Fatal(err)
			}

			handler := &WorkerHandler{}
			go handler.Handle(conn)
		}
	}()
	// run stage
	for i := 0; i < g.config.StageWorkers; i++ {
		flog.Infof("#%d stage run starting", i+1)
		go stage.Run(i+1, stopCh)
	}

	select {
	case <-stopCh:
		flog.Info("stop worker server")
	}
	return nil
}
