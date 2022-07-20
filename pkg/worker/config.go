package worker

import "github.com/tsundata/flowline/pkg/api/client/rest"

type Config struct {
	Host string
	Port int

	RestConfig *rest.Config

	StageWorkers int
}

func NewConfig() *Config {
	return &Config{RestConfig: &rest.Config{}}
}
