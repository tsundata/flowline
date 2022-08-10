package worker

import (
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/client/events"
	"github.com/tsundata/flowline/pkg/informer/informers"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"github.com/tsundata/flowline/pkg/worker/config"
	"github.com/tsundata/flowline/pkg/worker/sandbox"
	"golang.org/x/xerrors"
	"math/rand"
	"time"
)

type GenericWorkerServer struct {
	config *config.Config
	client client.Interface
}

func NewGenericWorkerServer(name string, config *config.Config) *GenericWorkerServer {
	flog.Infof("%s starting...", name)
	c, err := client.NewForConfig(config.RestConfig)
	if err != nil {
		panic(err)
	}

	sharedInformers := informers.NewSharedInformerFactory(c, ResyncPeriod(config)())
	config.InformerFactory = sharedInformers
	config.Runtime = sandbox.AvailableRuntime
	config.Client = c

	s := &GenericWorkerServer{
		config: config,
		client: c,
	}
	return s
}

func (g *GenericWorkerServer) Run(stopCh <-chan struct{}) error {
	cj, err := NewController(
		g.config,
		g.config.InformerFactory.Core().V1().Stages(),
		g.client,
	)
	if err != nil {
		return xerrors.Errorf("error new worker controller %v", err)
	}

	// Start events processing pipeline.
	g.config.EventBroadcaster.StartStructuredLogging("")
	g.config.EventBroadcaster.StartRecordingToSink(&events.EventSinkImpl{Interface: g.config.Client.EventsV1()})
	defer g.config.EventBroadcaster.Shutdown()

	// run controller
	ctx, _ := parallelizer.ContextForChannel(stopCh)
	go cj.Run(ctx, g.config.StageWorkers)

	// start informer
	g.config.InformerFactory.Start(stopCh)

	<-stopCh
	flog.Info("stop worker server")
	return nil
}

// ResyncPeriod returns a function which generates a duration each time it is
// invoked; this is so that multiple controllers don't get into lock-step and all
// hammer the apiserver with list requests simultaneously.
func ResyncPeriod(c *config.Config) func() time.Duration {
	return func() time.Duration {
		factor := rand.Float64() + 1
		return time.Duration(float64(c.MinResyncPeriod.Nanoseconds()) * factor)
	}
}
