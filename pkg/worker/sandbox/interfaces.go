package sandbox

type Interfaces interface {
	Name() string
	Run(code string, input interface{}) (output interface{}, err error) // todo runtime param
}
