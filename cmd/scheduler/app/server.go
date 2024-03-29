package app

import (
	"context"
	"fmt"
	"github.com/tsundata/flowline/cmd/scheduler/app/config"
	"github.com/tsundata/flowline/pkg/api/client/events"
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/scheduler"
	config2 "github.com/tsundata/flowline/pkg/scheduler/framework/config"
	"github.com/tsundata/flowline/pkg/scheduler/framework/runtime"
	"github.com/tsundata/flowline/pkg/scheduler/profile"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/signal"
	"github.com/tsundata/flowline/pkg/util/version"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

func NewSchedulerCommand() *cli.App {
	cli.VersionFlag = &cli.BoolFlag{
		Name:    "version",
		Aliases: []string{"V"},
		Usage:   "print only the version",
	}
	cli.VersionPrinter = func(_ *cli.Context) {
		fmt.Printf("version=%s\n", version.Version)
	}
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:  "load",
			Usage: "load yaml config",
		},
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:    "api-host",
			Aliases: []string{"A"},
			Value:   "127.0.0.1:5000",
			Usage:   "apiserver host",
			EnvVars: []string{"API_HOST"},
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:    "token",
			Aliases: []string{"T"},
			Usage:   "auth token",
			EnvVars: []string{"AUTH_TOKEN"},
		}),
	}
	return &cli.App{
		Name:                 "scheduler",
		Usage:                "scheduler server cli",
		EnableBashCompletion: true,
		Version:              version.Version,
		Before:               altsrc.InitInputSourceWithContext(flags, altsrc.NewYamlSourceFromFlagFunc("load")),
		Flags:                flags,
		Action: func(c *cli.Context) error {
			conf := &config.Config{
				ComponentConfig: config.Configuration{},
				RestConfig:      &rest.Config{},
			}
			conf.RestConfig.Host = c.String("api-host")
			conf.RestConfig.BearerToken = c.String("token")
			return runCommand(conf, signal.SetupSignalHandler())
		},
	}
}

func runCommand(c *config.Config, stopCh <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	c, sched, err := Setup(ctx, c)
	if err != nil {
		return err
	}

	return Run(ctx, c, sched)
}

// Option configures a framework.Registry.
type Option func(runtime.Registry) error

func Run(ctx context.Context, c *config.Config, sched *scheduler.Scheduler) error {
	flog.Info("scheduler running")

	// Start events processing pipeline.
	c.EventBroadcaster.StartRecordingToSink(ctx.Done())
	defer c.EventBroadcaster.Shutdown()

	// Start all informers
	c.InformerFactory.Start(ctx.Done())
	// Wait for all caches to sync before scheduling.
	c.InformerFactory.WaitForCacheSync(ctx.Done())

	sched.Run(ctx)
	return nil
}

func Setup(ctx context.Context, c *config.Config, outOfTreeRegistryOptions ...Option) (*config.Config, *scheduler.Scheduler, error) {
	outOfTreeRegistry := make(runtime.Registry)
	for _, option := range outOfTreeRegistryOptions {
		if err := option(outOfTreeRegistry); err != nil {
			return nil, nil, err
		}
	}

	c.Complete()

	recorderFactory := getRecorderFactory(c)

	completedProfiles := make([]config2.Profile, 0)
	sched, err := scheduler.New(
		c.Client,
		c.InformerFactory,
		recorderFactory,
		ctx.Done(),
		scheduler.WithConfig(c.RestConfig),
		//scheduler.WithProfiles(cc.ComponentConfig.Profiles...),
		scheduler.WithPercentageOfWorkersToScore(c.ComponentConfig.PercentageOfWorksToScore),
		scheduler.WithFrameworkOutOfTreeRegistry(outOfTreeRegistry),
		scheduler.WithStageMaxBackoffSeconds(c.ComponentConfig.StageMaxBackoffSeconds),
		scheduler.WithStageInitialBackoffSeconds(c.ComponentConfig.StageInitialBackoffSeconds),
		scheduler.WithStageMaxInUnschedulableStagesDuration(c.StageMaxInUnschedulableStagesDuration),
		scheduler.WithExtenders(c.ComponentConfig.Extenders...),
		scheduler.WithParallelism(c.ComponentConfig.Parallelism),
		scheduler.WithBuildFrameworkCapturer(func(profile config2.Profile) {
			completedProfiles = append(completedProfiles, profile)
		}),
	)
	if err != nil {
		return nil, nil, err
	}

	return c, sched, nil
}

func getRecorderFactory(cc *config.Config) profile.RecorderFactory {
	return func(name string) events.EventRecorder {
		return cc.EventBroadcaster.NewRecorder(name)
	}
}
