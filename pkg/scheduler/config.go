package scheduler

type Config struct {
	ApiURL string
}

func NewConfig() *Config {
	return &Config{}
}
