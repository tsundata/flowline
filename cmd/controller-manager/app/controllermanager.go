package app

import (
	"context"
	"github.com/tsundata/flowline/pkg/informer/informers"
	"github.com/tsundata/flowline/pkg/manager/config"
	"github.com/tsundata/flowline/pkg/manager/controller"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"github.com/tsundata/flowline/pkg/util/sets"
	"math/rand"
	"time"
)

func NewControllerInitializers() map[string]InitFunc {
	controllers := make(map[string]InitFunc)
	controllers["crontrigger"] = startCronTriggerController
	controllers["dag"] = startDagController
	controllers["stage"] = startStageController
	return controllers
}

func CreateControllerContext(c *config.Config, _ <-chan struct{}) (ControllerContext, error) {
	sharedInformers := informers.NewSharedInformerFactory(c.Client, ResyncPeriod(c)())

	ctx := ControllerContext{
		InformerFactory:    sharedInformers,
		ComponentConfig:    c,
		AvailableResources: nil,
		InformersStarted:   make(chan struct{}),
		ResyncPeriod:       ResyncPeriod(c),
	}
	return ctx, nil
}

const (
	// ControllerStartJitter is the Jitter used when starting controller managers
	ControllerStartJitter = 1.0
)

func StartControllers(ctx context.Context, controllerCtx ControllerContext, controllers map[string]InitFunc) error {
	for controllerName, initFn := range controllers {
		if !controllerCtx.IsControllerEnabled(controllerName) {
			flog.Warnf("%q is disabled", controllerName)
			continue
		}

		time.Sleep(parallelizer.Jitter(controllerCtx.ComponentConfig.Generic.ControllerStartInterval, ControllerStartJitter))

		flog.Infof("starting %s", controllerName)
		ctrl, started, err := initFn(ctx, controllerCtx)
		if err != nil {
			flog.Errorf("Error starting %q", controllerName)
			return err
		}
		if !started {
			flog.Warnf("Skipping %q", controllerName)
			continue
		}
		if ctrl != nil {
			flog.Infof("Started %q", controllerName)
		}
	}
	return nil
}

// ResyncPeriod returns a function which generates a duration each time it is
// invoked; this is so that multiple controllers don't get into lock-step and all
// hammer the apiserver with list requests simultaneously.
func ResyncPeriod(c *config.Config) func() time.Duration {
	return func() time.Duration {
		factor := rand.Float64() + 1
		return time.Duration(float64(c.Generic.MinResyncPeriod.Nanoseconds()) * factor)
	}
}

// ControllerInitializersFunc is used to create a collection of initializers
//  given the loopMode.
type ControllerInitializersFunc func() (initializers map[string]InitFunc)

// ControllerContext defines the context object for controller
type ControllerContext struct {

	// InformerFactory gives access to informers for the controller.
	InformerFactory informers.SharedInformerFactory

	// ComponentConfig provides access to init options for a given controller
	ComponentConfig *config.Config

	// AvailableResources is a map listing currently available resources
	AvailableResources map[schema.GroupVersionResource]bool

	// InformersStarted is closed after all of the controllers have been initialized and are running.  After this point it is safe,
	// for an individual controller to start the shared informers. Before it is closed, they should not.
	InformersStarted chan struct{}

	// ResyncPeriod generates a duration each time it is invoked; this is so that
	// multiple controllers don't get into lock-step and all hammer the apiserver
	// with list requests simultaneously.
	ResyncPeriod func() time.Duration
}

func (c ControllerContext) IsControllerEnabled(name string) bool {
	return IsControllerEnabled(name, ControllersDisabledByDefault, c.ComponentConfig.Generic.Controllers)
}

// ControllersDisabledByDefault is the set of controllers which is disabled by default
var ControllersDisabledByDefault = sets.NewString()

// IsControllerEnabled check if a specified controller enabled or not.
func IsControllerEnabled(name string, disabledByDefaultControllers sets.String, controllers []string) bool {
	hasStar := false
	for _, ctrl := range controllers {
		if ctrl == name {
			return true
		}
		if ctrl == "-"+name {
			return false
		}
		if ctrl == "*" {
			hasStar = true
		}
	}
	// if we get here, there was no explicit choice
	if !hasStar {
		// nothing on by default
		return false
	}

	return !disabledByDefaultControllers.Has(name)
}

// InitFunc is used to launch a particular controller. It returns a controller
// that can optionally implement other interfaces so that the controller manager
// can support the requested features.
// The returned controller may be nil, which will be considered an anonymous controller
// that requests no additional features from the controller manager.
// Any error returned will cause the controller process to `Fatal`
// The bool indicates whether the controller was enabled.
type InitFunc func(ctx context.Context, controllerCtx ControllerContext) (controller controller.Interface, enabled bool, err error)
