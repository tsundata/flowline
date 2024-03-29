package app

import (
	"context"
	"github.com/tsundata/flowline/pkg/controller/dag"
	"github.com/tsundata/flowline/pkg/manager/controller"
	"golang.org/x/xerrors"
)

func startDagController(ctx context.Context, controllerContext ControllerContext) (controller.Interface, bool, error) {
	cj, err := dag.NewController(
		controllerContext.InformerFactory.Core().V1().Jobs(),
		controllerContext.ComponentConfig.Client,
	)
	if err != nil {
		return nil, true, xerrors.Errorf("error creating dag controller %v", err)
	}

	go cj.Run(ctx, int(controllerContext.ComponentConfig.ConcurrentDagSyncs))

	return nil, false, nil
}
