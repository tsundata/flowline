package app

import (
	"context"
	"fmt"
	"github.com/tsundata/flowline/pkg/controller/stage"
	"github.com/tsundata/flowline/pkg/manager/controller"
)

func startStageController(ctx context.Context, controllerContext ControllerContext) (controller.Interface, bool, error) {
	cj, err := stage.NewController(
		controllerContext.InformerFactory.Core().V1().Stages(),
		controllerContext.ComponentConfig.Client,
	)
	if err != nil {
		return nil, true, fmt.Errorf("error creating stage controller %v", err)
	}

	go cj.Run(ctx, int(controllerContext.ComponentConfig.ConcurrentStageSyncs))

	return nil, false, nil
}
