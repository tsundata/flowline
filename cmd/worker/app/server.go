package app

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/signal"
	"github.com/tsundata/flowline/pkg/util/version"
	"github.com/tsundata/flowline/pkg/worker"
	"github.com/tsundata/flowline/pkg/worker/config"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

func NewWorkerCommand() *cli.App {
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
			Name:    "worker-id",
			Aliases: []string{"I"},
			Usage:   "worker id",
			EnvVars: []string{"WORKER_ID"},
		}),
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
		Name:    "worker",
		Usage:   "worker server cli",
		Version: version.Version,
		Before:  altsrc.InitInputSourceWithContext(flags, altsrc.NewYamlSourceFromFlagFunc("load")),
		Flags:   flags,
		Action: func(c *cli.Context) error {
			conf := config.NewConfig()
			conf.WorkerID = c.String("worker-id")
			conf.RestConfig.Host = c.String("api-host")
			conf.RestConfig.BearerToken = c.String("token")
			conf.StageWorkers = 1
			return Run(conf, signal.SetupSignalHandler())
		},
	}
}

func Run(c *config.Config, stopCh <-chan struct{}) error {
	flog.Info("worker running")

	server, err := CreateServerChain(c)
	if err != nil {
		return err
	}

	return server.Run(stopCh)
}

func CreateServerChain(c *config.Config) (*worker.Instance, error) {
	return worker.NewInstance(c), nil
}
