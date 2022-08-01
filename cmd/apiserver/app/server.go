package app

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/apiserver"
	"github.com/tsundata/flowline/pkg/apiserver/config"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/signal"
	"github.com/tsundata/flowline/pkg/util/version"
	"github.com/urfave/cli/v2"
	"time"
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
			conf := config.NewConfig()
			conf.BuildHandlerChainFunc = apiserver.DefaultBuildHandlerChain
			conf.Host = c.String("host")
			conf.Port = c.Int("port")
			conf.EnableIndex = true
			conf.JWTSecret = "abc" // fixme

			conf.HTTPReadTimeout = 10 * time.Second
			conf.HTTPWriteTimeout = 10 * time.Second
			conf.HTTPMaxHeaderBytes = 1 << 20

			return Run(conf, signal.SetupSignalHandler())
		},
	}
}

func Run(c *config.Config, stopCh <-chan struct{}) error {
	flog.Info("apiserver running")

	server, err := CreateServerChain(c)
	if err != nil {
		return err
	}

	return server.Run(stopCh)
}

func CreateServerChain(c *config.Config) (*apiserver.Instance, error) {
	return apiserver.NewInstance(c), nil
}
