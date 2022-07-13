package app

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/signal"
	"github.com/tsundata/flowline/pkg/util/version"
	"github.com/tsundata/flowline/pkg/worker"
	"github.com/urfave/cli/v2"
)

func NewWorkerCommand() *cli.App {
	cli.VersionFlag = &cli.BoolFlag{
		Name:    "version",
		Aliases: []string{"V"},
		Usage:   "print only the version",
	}
	cli.VersionPrinter = func(cCtx *cli.Context) {
		fmt.Printf("version=%s\n", cCtx.App.Version)
	}
	return &cli.App{
		Name:    "worker",
		Usage:   "worker server cli",
		Version: version.Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "host",
				Aliases: []string{"H"},
				Value:   "127.0.0.1",
				Usage:   "server host",
				EnvVars: []string{"CONTROLLER_MANAGER_HOST"},
			},
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"P"},
				Value:   5001,
				Usage:   "server port",
				EnvVars: []string{"CONTROLLER_MANAGER_PORT"},
			},
			&cli.StringFlag{
				Name:    "api-url",
				Aliases: []string{"A"},
				Value:   "http://127.0.0.1:5000/apis/",
				Usage:   "server host",
				EnvVars: []string{"CONTROLLER_MANAGER_HOST"},
			},
		},
		Action: func(c *cli.Context) error {
			config := worker.NewConfig() // todo
			config.Host = c.String("host")
			config.Port = c.Int("port")
			config.ApiURL = c.String("api-url")
			config.StageWorkers = 10
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

func Run(c *worker.Config, stopCh <-chan struct{}) error {
	flog.Info("worker running")

	server, err := CreateServerChain(c)
	if err != nil {
		return err
	}

	return server.Run(stopCh)
}

func CreateServerChain(c *worker.Config) (*worker.Instance, error) {
	return worker.NewInstance(c), nil
}
