package config

import (
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/informer/informers"
	"github.com/tsundata/flowline/pkg/scheduler"
	"github.com/tsundata/flowline/pkg/scheduler/framework/config"
	"time"
)

// Configuration configures a scheduler
type Configuration struct {
	// Parallelism defines the amount of parallelism in algorithms for scheduling a Stages. Must be greater than 0. Defaults to 16
	Parallelism int32

	// ClientConnection specifies the kubeconfig file and client connection
	// settings for the proxy server to use when communicating with the apiserver.
	ClientConnection interface{} //componentbaseconfig.ClientConnectionConfiguration
	// HealthzBindAddress is the IP address and port for the health check server to serve on.
	HealthzBindAddress string

	// PercentageOfWorksToScore is the percentage of all workers that once found feasible
	// for running a stage, the scheduler stops its search for more feasible workers in
	// the cluster. This helps improve scheduler's performance. Scheduler always tries to find
	// at least "minFeasibleWorkersToFind" feasible workers no matter what the value of this flag is.
	// Example: if the cluster size is 500 workers and the value of this flag is 30,
	// then scheduler stops finding further feasible workers once it finds 150 feasible ones.
	// When the value is 0, default percentage (5%--50% based on the size of the cluster) of the
	// workers will be scored.
	PercentageOfWorksToScore int32

	// StageInitialBackoffSeconds is the initial backoff for unschedulable stages.
	// If specified, it must be greater than 0. If this value is null, the default value (1s)
	// will be used.
	StageInitialBackoffSeconds int64

	// StageMaxBackoffSeconds is the max backoff for unschedulable stages.
	// If specified, it must be greater than or equal to stageInitialBackoffSeconds. If this value is null,
	// the default value (10s) will be used.
	StageMaxBackoffSeconds int64

	// Profiles are scheduling profiles that kube-scheduler supports. Stages can
	// choose to be scheduled under a particular profile by setting its associated
	// scheduler name. Stages that don't specify any scheduler name are scheduled
	// with the "default-scheduler" profile, if present here.
	Profiles []config.Profile

	// Extenders are the list of scheduler extenders, each holding the values of how to communicate
	// with the extender. These extenders are shared by all scheduler profiles.
	Extenders []config.Extender
}

// Config has all the context to run a Scheduler
type Config struct {
	// ComponentConfig is the scheduler server's configuration object.
	ComponentConfig Configuration

	// LoopbackClientConfig is a config for a privileged loopback connection
	LoopbackClientConfig interface{} //*restclient.Config

	//Authentication apiserver.AuthenticationInfo
	//Authorization  apiserver.AuthorizationInfo
	//SecureServing  *apiserver.SecureServingInfo

	Client          *client.RestClient
	Config          *scheduler.Config
	InformerFactory informers.SharedInformerFactory
	//DynInformerFactory dynamicinformer.DynamicSharedInformerFactory

	// StageMaxInUnschedulableStagesDuration is the maximum time a stage can stay in
	// unschedulableStages. If a stage stays in unschedulableStages for longer than this
	// value, the stage will be moved from unschedulableStages to backoffQ or activeQ.
	// If this value is empty, the default value (5min) will be used.
	StageMaxInUnschedulableStagesDuration time.Duration
}

func (c *Config) Complete() {
	// todo AuthorizeClientBearerToken

	c.Client = client.NewRestClient(c.Config.ApiURL)
	c.InformerFactory = informers.NewSharedInformerFactory(c.Client, 10*time.Minute)
}
