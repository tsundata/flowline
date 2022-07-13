package javascript

import (
	"github.com/robertkrimen/otto"
	_ "github.com/robertkrimen/otto/underscore"
	"github.com/tsundata/flowline/pkg/util/flog"
)

type Runtime struct {
	vm *otto.Otto
}

func NewRuntime() *Runtime {
	vm := otto.New()
	return &Runtime{
		vm: vm,
	}
}

func (r *Runtime) Name() string {
	return "javascript"
}

func (r *Runtime) Run(code []byte, input interface{}) (output interface{}, err error) {
	err = r.vm.Set("input", func(call otto.FunctionCall) otto.Value {
		val, err := r.vm.ToValue(input)
		if err != nil {
			flog.Error(err)
			return otto.NullValue()
		}
		return val
	})
	if err != nil {
		return nil, err
	}
	val, err := r.vm.Run(code)
	if err != nil {
		return nil, err
	}
	return val.Export()
}
