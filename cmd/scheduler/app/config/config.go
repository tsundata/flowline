package config

import (
	"github.com/tsundata/flowline/pkg/scheduler"
	"github.com/tsundata/flowline/pkg/scheduler/framework/config"
	"time"
)

// Configuration configures a scheduler
type Configuration struct {
	// Parallelism defines the amount of parallelism in algorithms for scheduling a Pods. Must be greater than 0. Defaults to 16
	Parallelism int32

	// ClientConnection specifies the kubeconfig file and client connection
	// settings for the proxy server to use when communicating with the apiserver.
	ClientConnection interface{} //componentbaseconfig.ClientConnectionConfiguration
	// HealthzBindAddress is the IP address and port for the health check server to serve on.
	HealthzBindAddress string

	// PercentageOfNodesToScore is the percentage of all nodes that once found feasible
	// for running a pod, the scheduler stops its search for more feasible nodes in
	// the cluster. This helps improve scheduler's performance. Scheduler always tries to find
	// at least "minFeasibleNodesToFind" feasible nodes no matter what the value of this flag is.
	// Example: if the cluster size is 500 nodes and the value of this flag is 30,
	// then scheduler stops finding further feasible nodes once it finds 150 feasible ones.
	// When the value is 0, default percentage (5%--50% based on the size of the cluster) of the
	// nodes will be scored.
	PercentageOfNodesToScore int32

	// PodInitialBackoffSeconds is the initial backoff for unschedulable pods.
	// If specified, it must be greater than 0. If this value is null, the default value (1s)
	// will be used.
	PodInitialBackoffSeconds int64

	// PodMaxBackoffSeconds is the max backoff for unschedulable pods.
	// If specified, it must be greater than or equal to podInitialBackoffSeconds. If this value is null,
	// the default value (10s) will be used.
	PodMaxBackoffSeconds int64

	// Profiles are scheduling profiles that kube-scheduler supports. Pods can
	// choose to be scheduled under a particular profile by setting its associated
	// scheduler name. Pods that don't specify any scheduler name are scheduled
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

	Client interface{}
	Config *scheduler.Config
	//InformerFactory    informers.SharedInformerFactory
	//DynInformerFactory dynamicinformer.DynamicSharedInformerFactory

	//nolint:staticcheck // SA1019 this deprecated field still needs to be used for now. It will be removed once the migration is done.
	EventBroadcaster interface{}

	// PodMaxInUnschedulablePodsDuration is the maximum time a pod can stay in
	// unschedulablePods. If a pod stays in unschedulablePods for longer than this
	// value, the pod will be moved from unschedulablePods to backoffQ or activeQ.
	// If this value is empty, the default value (5min) will be used.
	PodMaxInUnschedulablePodsDuration time.Duration
}
