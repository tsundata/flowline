package app

import (
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver"
	"github.com/tsundata/flowline/pkg/apiserver/config"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/signal"
	"github.com/tsundata/flowline/pkg/util/version"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
	"golang.org/x/xerrors"
	"time"
)

func NewAPIServerCommand() *cli.App {
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
			Name:    "host",
			Aliases: []string{"H"},
			Value:   "127.0.0.1",
			Usage:   "server host",
			EnvVars: []string{"APISERVER_HOST"},
		}),
		altsrc.NewIntFlag(&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"P"},
			Value:   5000,
			Usage:   "server port",
			EnvVars: []string{"APISERVER_PORT"},
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:    "secret",
			Aliases: []string{"S"},
			Usage:   "jwt secret",
			EnvVars: []string{"APISERVER_SECRET"},
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:    "etcd",
			Aliases: []string{"E"},
			Usage:   "etcd server host",
			EnvVars: []string{"APISERVER_ETCD"},
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:    "user",
			Aliases: []string{"U"},
			Usage:   "user uid",
		}),
	}
	return &cli.App{
		Name:                 "apiserver",
		Usage:                "api server cli",
		EnableBashCompletion: true,
		Version:              version.Version,
		Before:               altsrc.InitInputSourceWithContext(flags, altsrc.NewYamlSourceFromFlagFunc("load")),
		Flags:                flags,
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
				Name:  "token",
				Usage: "generate token",
				Action: func(c *cli.Context) error {
					if c.String("user") == "" {
						return xerrors.New("error user")
					}
					var jc = jwt.NewWithClaims(
						jwt.SigningMethodHS512,
						&meta.UserClaims{
							RegisteredClaims: &jwt.RegisteredClaims{
								ID: c.String("user"),
							},
						},
					)
					if c.String("secret") == "" {
						return xerrors.New("error secret")
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
