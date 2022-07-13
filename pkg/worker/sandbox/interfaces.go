package sandbox

type Interfaces interface {
	Name() string
	Run(code []byte, input interface{}) (output interface{}, err error) // todo runtime param
}
