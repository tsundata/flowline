package registry

import (
	"context"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/watch"
)

type decoratedWatcher struct {
	w         watch.Interface
	decorator func(runtime.Object)
	cancel    context.CancelFunc
	resultCh  chan watch.Event
}

func newDecoratedWatcher(ctx context.Context, w watch.Interface, decorator func(runtime.Object)) *decoratedWatcher {
	ctx, cancel := context.WithCancel(ctx)
	d := &decoratedWatcher{
		w:         w,
		decorator: decorator,
		cancel:    cancel,
		resultCh:  make(chan watch.Event),
	}
	go d.run(ctx)
	return d
}

// run decorates watch events from the underlying watcher until its result channel
// is closed or the passed in context is done.
// When run() returns, decoratedWatcher#resultCh is closed.
func (d *decoratedWatcher) run(ctx context.Context) {
	var recv, send watch.Event
	var ok bool
	defer close(d.resultCh)
	for {
		select {
		case recv, ok = <-d.w.ResultChan():
			if !ok {
				// The underlying channel was closed, cancel our context
				d.cancel()
				return
			}
			switch recv.Type {
			case watch.Added, watch.Modified, watch.Deleted, watch.Bookmark:
				d.decorator(recv.Object)
				send = recv
			case watch.Error:
				send = recv
			}
			select {
			case d.resultCh <- send:
				// propagated event successfully
			case <-ctx.Done():
				// context timed out or was cancelled, stop the underlying watcher
				d.w.Stop()
				return
			}
		case <-ctx.Done():
			// context timed out or was cancelled, stop the underlying watcher
			d.w.Stop()
			return
		}
	}
}

func (d *decoratedWatcher) Stop() {
	// stop the underlying watcher
	d.w.Stop()
	// cancel our context
	d.cancel()
}

func (d *decoratedWatcher) ResultChan() <-chan watch.Event {
	return d.resultCh
}
