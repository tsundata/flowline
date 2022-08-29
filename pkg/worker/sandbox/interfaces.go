package sandbox

type Interfaces interface {
	Name() string
	Run(code string, input interface{}, variables map[string]string) (output interface{}, err error)
}
