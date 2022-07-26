package config

import (
	"net/http"
	"time"
)

type Config struct {
	Host string
	Port int

	EtcdHost     string
	EtcdUsername string
	EtcdPassword string

	JWTSecret string

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
	}
}
