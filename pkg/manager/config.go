package manager

import "github.com/tsundata/flowline/pkg/api/client/rest"

type Config struct {
	RestConfig *rest.Config
}

func NewConfig() *Config {
	return &Config{RestConfig: &rest.Config{}}
}
