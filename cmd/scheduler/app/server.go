package app

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/scheduler"
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
				Name:    "api-url",
				Aliases: []string{"A"},
				Value:   "http://127.0.0.1:5000/",
				Usage:   "server host",
				EnvVars: []string{"CONTROLLER_MANAGER_HOST"},
			},
		},
		Action: func(c *cli.Context) error {
			config := scheduler.NewConfig() // todo
			config.ApiURL = c.String("api-url")
			return Run(config, signal.SetupSignalHandler())
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

func Run(c *scheduler.Config, stopCh <-chan struct{}) error {
	flog.Info("scheduler running")

	server, err := CreateServerChain(c)
	if err != nil {
		return err
	}

	return server.Run(stopCh)
}

func CreateServerChain(c *scheduler.Config) (*scheduler.Instance, error) {
	return scheduler.NewInstance(c), nil
}
