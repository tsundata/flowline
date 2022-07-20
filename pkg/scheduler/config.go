package scheduler

type Config struct {
	ApiHost string
}

func NewConfig() *Config {
	return &Config{}
}
