package config

import (
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/client/record"
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/informer/informers"
	"github.com/tsundata/flowline/pkg/runtime"
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

	Client client.Interface

	EventBroadcaster record.EventBroadcaster
	EventRecorder    record.EventRecorder
}

func NewConfig() *Config {
	eventBroadcaster := record.NewBroadcaster()
	eventRecorder := eventBroadcaster.NewRecorder(runtime.NewScheme(), meta.EventSource{Component: "worker"})
	return &Config{
		RestConfig:       &rest.Config{},
		StageWorkers:     2,
		EventBroadcaster: eventBroadcaster,
		EventRecorder:    eventRecorder,
	}
}
