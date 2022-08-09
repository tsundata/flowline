package app

import (
	"context"
	"fmt"
	"github.com/tsundata/flowline/pkg/controller/job"
	"github.com/tsundata/flowline/pkg/manager/controller"
)

func startJobController(ctx context.Context, controllerContext ControllerContext) (controller.Interface, bool, error) {
	cj, err := job.NewController(
		controllerContext.InformerFactory.Core().V1().Stages(),
		controllerContext.ComponentConfig.Client,
	)
	if err != nil {
		return nil, true, fmt.Errorf("error creating job controller %v", err)
	}

	go cj.Run(ctx, int(controllerContext.ComponentConfig.ConcurrentJobSyncs))

	return nil, false, nil
}
