package etcd

import (
	"context"
	"sync"
)

type priorityAndFairnessKeyType int

const (
	// priorityAndFairnessInitializationSignalKey is a key under which
	// initialization signal function for watch requests is stored
	// in the context.
	priorityAndFairnessInitializationSignalKey priorityAndFairnessKeyType = iota
)

// initializationSignalFrom returns an initialization signal function
// which when called signals that watch initialization has already finished
// to priority and fairness dispatcher.
func initializationSignalFrom(ctx context.Context) (InitializationSignal, bool) {
	signal, ok := ctx.Value(priorityAndFairnessInitializationSignalKey).(InitializationSignal)
	return signal, ok && signal != nil
}

// WatchInitialized sends a signal to priority and fairness dispatcher
// that a given watch request has already been initialized.
func WatchInitialized(ctx context.Context) {
	if signal, ok := initializationSignalFrom(ctx); ok {
		signal.Signal()
	}
}

// InitializationSignal is an interface that allows sending and handling
// initialization signals.
type InitializationSignal interface {
	// Signal notifies the dispatcher about finished initialization.
	Signal()
	// Wait waits for the initialization signal.
	Wait()
}

type initializationSignal struct {
	once sync.Once
	done chan struct{}
}

func NewInitializationSignal() InitializationSignal {
	return &initializationSignal{
		once: sync.Once{},
		done: make(chan struct{}),
	}
}

func (i *initializationSignal) Signal() {
	i.once.Do(func() { close(i.done) })
}

func (i *initializationSignal) Wait() {
	<-i.done
}
