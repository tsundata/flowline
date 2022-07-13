package worker

import (
	"bufio"
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/worker/sandbox"
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

		obj := meta.Function{}
		_, _, err = codec.Decode(message, nil, &obj)
		if err != nil {
			flog.Error(err)
			continue
		}

		rt := sandbox.Factory(sandbox.RuntimeType(obj.Runtime))
		out, err := rt.Run(obj.Code, 1000)
		if err != nil {
			flog.Error(err)
			continue
		}
		fmt.Println(out)

		_, _ = conn.Write([]byte("OK"))
	}
}
