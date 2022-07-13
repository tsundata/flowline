package worker

type Config struct {
	Host string
	Port int

	ApiURL string
}

func NewConfig() *Config {
	return &Config{}
}
