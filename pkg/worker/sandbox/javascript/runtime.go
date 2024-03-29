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

func (r *Runtime) Run(code string, input interface{}, variables map[string]string) (output interface{}, err error) {
	// input function
	err = r.vm.Set("input", func(call otto.FunctionCall) otto.Value {
		val, err := r.vm.ToValue(input)
		t := call.Argument(0).String()
		var defaultValue interface{}
		switch t {
		case "string":
			defaultValue = ""
		case "number":
			defaultValue = 0
		case "bool":
			defaultValue = false
		default:
			defaultValue = nil
		}

		if err != nil {
			flog.Error(err)
			val, _ = otto.ToValue(defaultValue)
		}
		if input == nil {
			val, _ = otto.ToValue(defaultValue)
		}

		return val
	})
	if err != nil {
		return nil, err
	}
	// variable function
	err = r.vm.Set("variable", func(call otto.FunctionCall) otto.Value {
		key := call.Argument(0).String()
		value, ok := variables[key]
		if !ok {
			return otto.NullValue()
		}

		val, _ := otto.ToValue(value)
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
