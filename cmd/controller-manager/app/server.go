package app

import (
	"context"
	"fmt"
	"github.com/tsundata/flowline/pkg/manager/config"
	"github.com/tsundata/flowline/pkg/manager/controller"
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
	cli.VersionPrinter = func(cCtx *cli.Context) {
		fmt.Printf("version=%s\n", cCtx.App.Version)
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
				Usage:   "server host",
				EnvVars: []string{"CONTROLLER_MANAGER_HOST"},
			},
		},
		Action: func(c *cli.Context) error {
			conf := config.NewConfig()
			conf.RestConfig.Host = c.String("api-url")
			return Run(conf, signal.SetupSignalHandler())
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

// serviceAccountTokenControllerStarter is special because it must run first to set up permissions for other controllers.
// It cannot use the "normal" client builder, so it tracks its own. It must also avoid being included in the "normal"
// init map so that it can always run first.
type serviceAccountTokenControllerStarter struct {
	rootClientBuilder interface{}
}

func (s serviceAccountTokenControllerStarter) startServiceAccountTokenController() InitFunc {
	return func(ctx context.Context, controllerCtx ControllerContext) (controller controller.Interface, enabled bool, err error) {
		return nil, false, nil
	}
}

func Run(c *config.Config, stopCh <-chan struct{}) error {
	flog.Info("controller-manager running")

	saTokenControllerInitFunc := serviceAccountTokenControllerStarter{}.startServiceAccountTokenController()

	run := func(ctx context.Context, startSATokenController InitFunc, initializersFunc ControllerInitializersFunc) {
		controllerContext, err := CreateControllerContext(c, ctx.Done())
		if err != nil {
			flog.Fatalf("error building controller context: %v", err)
		}
		controllerInitializers := initializersFunc()
		if err := StartControllers(ctx, controllerContext, startSATokenController, controllerInitializers); err != nil {
			flog.Fatalf("error starting controllers: %v", err)
		}

		controllerContext.InformerFactory.Start(stopCh)
		close(controllerContext.InformersStarted)

		<-ctx.Done()
	}

	ctx, _ := parallelizer.ContextForChannel(stopCh)
	run(ctx, saTokenControllerInitFunc, NewControllerInitializers)

	<-stopCh
	return nil
}
