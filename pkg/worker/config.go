package worker

type Config struct {
	Host string
	Port int

	ApiURL string

	StageWorkers int
}

func NewConfig() *Config {
	return &Config{}
}
