package app

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/controlplane"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/signal"
	"github.com/tsundata/flowline/pkg/util/version"
	"github.com/urfave/cli/v2"
)

func NewAPIServerCommand() *cli.App {
	cli.VersionFlag = &cli.BoolFlag{
		Name:    "version",
		Aliases: []string{"V"},
		Usage:   "print only the version",
	}
	cli.VersionPrinter = func(cCtx *cli.Context) {
		fmt.Printf("version=%s\n", cCtx.App.Version)
	}
	return &cli.App{
		Name:    "apiserver",
		Usage:   "api server cli",
		Version: version.Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "host",
				Aliases: []string{"H"},
				Value:   "127.0.0.1",
				Usage:   "server host",
				EnvVars: []string{"APISERVER_HOST"},
			},
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"P"},
				Value:   5000,
				Usage:   "server port",
				EnvVars: []string{"APISERVER_PORT"},
			},
		},
		Action: func(c *cli.Context) error {
			config := controlplane.NewConfig() // todo
			config.Host = c.String("host")
			config.Port = c.Int("port")
			config.EnableIndex = true
			return Run(config, signal.SetupSignalHandler())
		},
		Commands: []*cli.Command{
			{
				Name:    "info",
				Aliases: []string{"I"},
				Usage:   "print info",
				Action: func(cCtx *cli.Context) error {
					fmt.Println("apiserver")
					return nil
				},
			},
		},
	}
}

func Run(c *controlplane.Config, stopCh <-chan struct{}) error {
	flog.Info("apiserver running")

	server, err := CreateServerChain(c)
	if err != nil {
		return err
	}

	return server.Run(stopCh)
}

func CreateServerChain(c *controlplane.Config) (*controlplane.Instance, error) {
	return controlplane.NewInstance(c), nil
}
