package app

import (
	"context"
	"fmt"
	"github.com/tsundata/flowline/pkg/controller/crontrigger"
	"github.com/tsundata/flowline/pkg/manager/controller"
)

func startCronTriggerController(ctx context.Context, controllerContext ControllerContext) (controller.Interface, bool, error) {
	cj, err := crontrigger.NewController(
		controllerContext.InformerFactory.Core().V1().Jobs(),
		controllerContext.InformerFactory.Core().V1().Workflows(),
		controllerContext.ComponentConfig.Client,
	)
	if err != nil {
		return nil, true, fmt.Errorf("error creating crontrigger controller %v", err)
	}

	go cj.Run(ctx, int(controllerContext.ComponentConfig.ConcurrentCronTriggerSyncs))

	return nil, false, nil
}
