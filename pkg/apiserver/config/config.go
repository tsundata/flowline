package config

import (
	"github.com/tsundata/flowline/pkg/apiserver/storage/config"
	"net/http"
	"time"
)

type Config struct {
	// server
	Host string
	Port int

	// etcd
	ETCDConfig config.TransportConfig

	// jwt
	JWTSecret string

	// cors
	CorsAllowedOriginPatterns []string

	// APIServerID is the ID of this API server
	APIServerID           string
	BuildHandlerChainFunc func(apiHandler http.Handler, c *Config) http.Handler

	EnableIndex bool

	HTTPReadTimeout    time.Duration
	HTTPWriteTimeout   time.Duration
	HTTPMaxHeaderBytes int
}

// NewConfig Default Config
func NewConfig() *Config {
	return &Config{
		CorsAllowedOriginPatterns: []string{".*"},
		ETCDConfig:                config.TransportConfig{},
	}
}
