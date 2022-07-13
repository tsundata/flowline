package stage

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/worker/queue"
	"github.com/tsundata/flowline/pkg/worker/sandbox"
)

func Run(i int, stopCh <-chan struct{}) {
	for stage := range queue.StageQueue {
		go func(i int, s *meta.Stage) {
			defer func() {
				if r := recover(); r != nil {
					flog.Errorf("#%d stage run error %v", i, r)
				}
			}()

			rt := sandbox.Factory(sandbox.RuntimeType(s.Runtime))
			out, err := rt.Run(s.Code, 1000)
			if err != nil {
				flog.Error(err)
				return
			}
			fmt.Println(i, out)
		}(i, stage)
	}
	select {
	case <-stopCh:
		flog.Infof("stop #%d stage run", i)
	}
}
