package sandbox

import "github.com/tsundata/flowline/pkg/worker/sandbox/javascript"

type RuntimeType string

const (
	RuntimeGo         RuntimeType = "go"
	RuntimeJavaScript RuntimeType = "javascript"
)

var AvailableRuntime = []string{
	string(RuntimeGo),
	string(RuntimeJavaScript),
}

func Factory(rt RuntimeType) Interfaces {
	switch rt {
	case RuntimeJavaScript:
		return javascript.NewRuntime()
	}
	return nil
}
