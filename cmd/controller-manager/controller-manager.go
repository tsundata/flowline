package main

import (
	"github.com/tsundata/flowline/cmd/controller-manager/app"
	"github.com/tsundata/flowline/pkg/util/flog"
	"os"
)

func main() {
	command := app.NewControllerManagerCommand()
	if err := command.Run(os.Args); err != nil {
		flog.Fatal(err)
	}
}
