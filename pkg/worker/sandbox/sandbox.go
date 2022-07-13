package sandbox

import "github.com/tsundata/flowline/pkg/worker/sandbox/javascript"

type RuntimeType string

const (
	RuntimeJavaScript RuntimeType = "javascript"
)

func Factory(rt RuntimeType) Interfaces {
	switch rt {
	case RuntimeJavaScript:
		return javascript.NewRuntime()
	}
	return nil
}
