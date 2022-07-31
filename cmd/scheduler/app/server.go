package app

import (
	"context"
	"fmt"
	"github.com/tsundata/flowline/cmd/scheduler/app/config"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/scheduler"
	config2 "github.com/tsundata/flowline/pkg/scheduler/framework/config"
	"github.com/tsundata/flowline/pkg/scheduler/framework/runtime"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/signal"
	"github.com/tsundata/flowline/pkg/util/version"
	"github.com/urfave/cli/v2"
)

func NewSchedulerCommand() *cli.App {
	cli.VersionFlag = &cli.BoolFlag{
		Name:    "version",
		Aliases: []string{"V"},
		Usage:   "print only the version",
	}
	cli.VersionPrinter = func(cCtx *cli.Context) {
		fmt.Printf("version=%s\n", cCtx.App.Version)
	}
	return &cli.App{
		Name:    "scheduler",
		Usage:   "scheduler server cli",
		Version: version.Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "api-host",
				Aliases: []string{"A"},
				Value:   "127.0.0.1:5000",
				Usage:   "apiserver host",
				EnvVars: []string{"CONTROLLER_MANAGER_HOST"},
			},
		},
		Action: func(c *cli.Context) error {
			cc := &config.Config{
				ComponentConfig: config.Configuration{},
				Config:          &scheduler.Config{},
			} // todo
			cc.Config.ApiHost = c.String("api-host")
			return runCommand(cc, signal.SetupSignalHandler())
		},
		Commands: []*cli.Command{
			{
				Name:    "info",
				Aliases: []string{"I"},
				Usage:   "print info",
				Action: func(cCtx *cli.Context) error {
					fmt.Println("manager")
					return nil
				},
			},
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

	var err error
	c.Client, err = client.NewForConfig(&rest.Config{
		Host:          c.Config.ApiHost,
		ContentConfig: rest.ContentConfig{},
		Impersonate:   rest.ImpersonationConfig{},
	})
	if err != nil {
		return nil, nil, err
	}

	c.Complete()

	completedProfiles := make([]config2.Profile, 0)
	sche, err := scheduler.New(
		c.Client,
		c.InformerFactory,
		nil,
		nil,
		ctx.Done(),
		scheduler.WithConfig(c.Config),
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

	return c, sche, nil
}
