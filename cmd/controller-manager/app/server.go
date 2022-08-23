package app

import (
	"context"
	"fmt"
	"github.com/tsundata/flowline/pkg/api/client/events"
	"github.com/tsundata/flowline/pkg/manager/config"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"github.com/tsundata/flowline/pkg/util/signal"
	"github.com/tsundata/flowline/pkg/util/version"
	"github.com/urfave/cli/v2"
)

func NewControllerManagerCommand() *cli.App {
	cli.VersionFlag = &cli.BoolFlag{
		Name:    "version",
		Aliases: []string{"V"},
		Usage:   "print only the version",
	}
	cli.VersionPrinter = func(_ *cli.Context) {
		fmt.Printf("version=%s\n", version.Version)
	}
	return &cli.App{
		Name:    "controller-manager",
		Usage:   "controller manager server cli",
		Version: version.Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "api-url",
				Aliases: []string{"A"},
				Value:   "127.0.0.1:5000",
				Usage:   "apiserver host",
				EnvVars: []string{"API_HOST"},
			},
			&cli.StringFlag{
				Name:    "token",
				Aliases: []string{"T"},
				Usage:   "auth token",
				EnvVars: []string{"AUTH_TOKEN"},
			},
		},
		Action: func(c *cli.Context) error {
			conf := config.NewConfig()
			conf.RestConfig.Host = c.String("api-url")
			conf.RestConfig.BearerToken = c.String("token")
			return Run(conf.Complete(), signal.SetupSignalHandler())
		},
	}
}

func Run(c *config.Config, stopCh <-chan struct{}) error {
	flog.Info("controller-manager running")

	// Start events processing pipeline.
	c.EventBroadcaster.StartStructuredLogging("")
	c.EventBroadcaster.StartRecordingToSink(&events.EventSinkImpl{Interface: c.Client.EventsV1()})
	defer c.EventBroadcaster.Shutdown()

	run := func(ctx context.Context, initializersFunc ControllerInitializersFunc) {
		controllerContext, err := CreateControllerContext(c, ctx.Done())
		if err != nil {
			flog.Fatalf("error building controller context: %v", err)
		}
		controllerInitializers := initializersFunc()
		if err := StartControllers(ctx, controllerContext, controllerInitializers); err != nil {
			flog.Fatalf("error starting controllers: %v", err)
		}

		controllerContext.InformerFactory.Start(stopCh)
		close(controllerContext.InformersStarted)

		<-ctx.Done()
	}

	ctx, _ := parallelizer.ContextForChannel(stopCh)
	run(ctx, NewControllerInitializers)

	<-stopCh
	return nil
}
