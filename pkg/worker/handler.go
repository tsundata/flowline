package worker

import (
	"bufio"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/worker/queue"
	"net"
)

type WorkerHandler struct{}

func (w *WorkerHandler) Handle(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		message, err := reader.ReadBytes('\n')
		if err != nil {
			flog.Info("conn close")
			_ = conn.Close()
			return
		}
		flog.Info(string(message))

		codec := runtime.JsonCoder{}

		obj := meta.Stage{}
		_, _, err = codec.Decode(message, nil, &obj)
		if err != nil {
			flog.Error(err)
			continue
		}

		// enqueue
		queue.StageQueue <- &obj

		_, _ = conn.Write([]byte("OK"))
	}
}
