package main

import (
	"github.com/tsundata/flowline/cmd/apiserver/app"
	"github.com/tsundata/flowline/pkg/util/flog"
	"os"
)

func main() {
	command := app.NewAPIServerCommand()
	if err := command.Run(os.Args); err != nil {
		flog.Fatal(err)
	}
}
