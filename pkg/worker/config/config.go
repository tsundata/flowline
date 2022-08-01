package config

import (
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/informer/informers"
	"time"
)

type Config struct {
	WorkerID string
	Runtime  []string

	RestConfig *rest.Config

	// InformerFactory gives access to informers for the controller.
	InformerFactory informers.SharedInformerFactory
	// minResyncPeriod is the resync period in reflectors; will be random between
	// minResyncPeriod and 2*minResyncPeriod.
	MinResyncPeriod time.Duration

	StageWorkers int
}

func NewConfig() *Config {
	return &Config{RestConfig: &rest.Config{}}
}
