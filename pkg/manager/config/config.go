package config

import (
	"github.com/tsundata/flowline/pkg/api/client"
	v1 "github.com/tsundata/flowline/pkg/api/client/core/v1"
	"github.com/tsundata/flowline/pkg/api/client/record"
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/api/meta"
	"time"
)

type Config struct {
	RestConfig *rest.Config

	Client client.Interface

	Generic GenericControllerManagerConfiguration

	// ConcurrentCronJobSyncs is the number of cron job objects that are
	// allowed to sync concurrently. Larger number = more responsive jobs,
	// but more CPU (and network) load.
	ConcurrentCronTriggerSyncs int32
	ConcurrentDagSyncs         int32
	ConcurrentStageSyncs       int32
	ConcurrentJobSyncs         int32

	EventBroadcaster record.EventBroadcaster
	EventRecorder    record.EventRecorder
}

// GenericControllerManagerConfiguration holds configuration for a generic controller-manager
type GenericControllerManagerConfiguration struct {
	// port is the port that the controller-manager's http service runs on.
	Port int32
	// address is the IP address to serve on (set to 0.0.0.0 for all interfaces).
	Address string
	// minResyncPeriod is the resync period in reflectors; will be random between
	// minResyncPeriod and 2*minResyncPeriod.
	MinResyncPeriod time.Duration
	// How long to wait between starting controller managers
	ControllerStartInterval time.Duration
	// Controllers is the list of controllers to enable or disable
	// '*' means "all enabled by default controllers"
	// 'foo' means "enable 'foo'"
	// '-foo' means "disable 'foo'"
	// first item for a particular name wins
	Controllers []string
}

func NewConfig() *Config {
	eventBroadcaster := record.NewBroadcaster()
	eventRecorder := eventBroadcaster.NewRecorder(v1.Scheme, meta.EventSource{Component: "controller-manager"})
	return &Config{
		RestConfig: &rest.Config{
			DisableCompression: true,
			ContentConfig:      rest.ContentConfig{},
			Impersonate:        rest.ImpersonationConfig{},
		},
		Generic: GenericControllerManagerConfiguration{
			Controllers: []string{
				"*",
			},
		},
		ConcurrentCronTriggerSyncs: 2,
		ConcurrentDagSyncs:         2,
		ConcurrentStageSyncs:       2,
		ConcurrentJobSyncs:         2,
		EventBroadcaster:           eventBroadcaster,
		EventRecorder:              eventRecorder,
	}
}

func (c *Config) Complete() *Config {
	var err error
	c.Client, err = client.NewForConfig(c.RestConfig)
	if err != nil {
		panic(err)
	}
	return c
}
