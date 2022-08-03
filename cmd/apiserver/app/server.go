package app

import (
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/tsundata/flowline/pkg/api/meta"
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
			&cli.StringFlag{
				Name:    "secret",
				Aliases: []string{"S"},
				Usage:   "jwt secret",
				EnvVars: []string{"APISERVER_SECRET"},
			},
			&cli.StringFlag{
				Name:    "etcd",
				Aliases: []string{"E"},
				Usage:   "etcd server host",
				EnvVars: []string{"APISERVER_ETCD"},
			},
			&cli.StringFlag{
				Name:    "user",
				Aliases: []string{"U"},
				Usage:   "user uid",
			},
		},
		Action: func(c *cli.Context) error {
			conf := config.NewConfig()
			conf.BuildHandlerChainFunc = apiserver.DefaultBuildHandlerChain
			conf.Host = c.String("host")
			conf.Port = c.Int("port")
			conf.EnableIndex = true
			conf.JWTSecret = c.String("secret")
			conf.ETCDConfig.ServerList = []string{c.String("etcd")}

			conf.HTTPReadTimeout = 10 * time.Second
			conf.HTTPWriteTimeout = 10 * time.Second
			conf.HTTPMaxHeaderBytes = 1 << 20

			return Run(conf, signal.SetupSignalHandler())
		},
		Commands: []*cli.Command{
			{
				Name:    "token",
				Aliases: []string{"T"},
				Usage:   "generate token",
				Action: func(c *cli.Context) error {
					var jc = jwt.NewWithClaims(
						jwt.SigningMethodHS512,
						&meta.UserClaims{
							RegisteredClaims: &jwt.RegisteredClaims{
								ID: c.String("user"),
							},
						},
					)
					if c.String("secret") == "" {
						return errors.New("error secret")
					}
					token, err := jc.SignedString([]byte(c.String("secret")))
					if err != nil {
						return err
					}
					fmt.Println(token)
					return nil
				},
			},
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
