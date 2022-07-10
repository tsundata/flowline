package controlplane

import (
	"github.com/tsundata/flowline/pkg/controlplane/filters"
	"github.com/tsundata/flowline/pkg/controlplane/routes"
	"net/http"
)

const DefaultAPIPrefix = "/api"

type Config struct {
	Host string
	Port int

	// APIServerID is the ID of this API server
	APIServerID           string
	BuildHandlerChainFunc func(apiHandler http.Handler, c *Config) http.Handler

	EnableIndex bool
}

func NewConfig() *Config {
	return &Config{
		BuildHandlerChainFunc: DefaultBuildHandlerChain,
	}
}

//DefaultBuildHandlerChain set default filters
func DefaultBuildHandlerChain(apiHandler http.Handler, c *Config) http.Handler {
	handler := filters.WithCORS(apiHandler, nil, nil, nil, nil, "true")
	return handler
}

//installAPI install routes
func installAPI(s *GenericAPIServer, c *Config) {
	if c.EnableIndex {
		routes.Index{}.Install(s.Handler.NonRestfulMux)
	}
}
