package signal

import (
	"context"
	"os"
	"os/signal"
)

var onlyOneSignalHandler = make(chan struct{})
var shutdownHandler chan os.Signal

func SetupSignalHandler() <-chan struct{} {
	return SetupSignalContext().Done()
}

func SetupSignalContext() context.Context {
	close(onlyOneSignalHandler)

	shutdownHandler = make(chan os.Signal, 2)

	ctx, cancel := context.WithCancel(context.Background())
	signal.Notify(shutdownHandler, shutdownSignals...)
	go func() {
		<-shutdownHandler
		cancel()
		<-shutdownHandler
		os.Exit(1)
	}()

	return ctx
}
