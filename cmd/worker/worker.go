package main

import (
	"github.com/tsundata/flowline/cmd/worker/app"
	"github.com/tsundata/flowline/pkg/util/flog"
	"os"
)

func main() {
	command := app.NewWorkerCommand()
	if err := command.Run(os.Args); err != nil {
		flog.Fatal(err)
	}
}
