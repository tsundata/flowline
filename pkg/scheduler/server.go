package scheduler

import (
	"bytes"
	"fmt"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
	"net"
	"time"
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

		conn, err := net.Dial("tcp", "127.0.0.1:5001") //fixme
		if err != nil {
			flog.Error(err)
			return
		}

		buf := bytes.NewBufferString("")
		codec := runtime.JsonCoder{}
		err = codec.Encode(&meta.Stage{
			Runtime: "javascript",
			Code:    "input() + Math.floor(Date.now() / 1000)",
			Input:   1000,
		}, buf)
		if err != nil {
			flog.Error(err)
			return
		}
		buf.WriteByte('\n')

		// fixme
		for range time.Tick(time.Second) {
			flog.Info(buf.String())
			_, err = conn.Write(buf.Bytes())
			if err != nil {
				flog.Error(err)
				return
			}
		}
	}()
	select {
	case <-stopCh:
		flog.Info("stop scheduler server")
	}
	return nil
}
