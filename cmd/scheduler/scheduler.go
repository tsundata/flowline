package main

import (
	"github.com/tsundata/flowline/cmd/scheduler/app"
	"github.com/tsundata/flowline/pkg/util/flog"
	"os"
)

func main() {
	command := app.NewSchedulerCommand()
	if err := command.Run(os.Args); err != nil {
		flog.Fatal(err)
	}
}
