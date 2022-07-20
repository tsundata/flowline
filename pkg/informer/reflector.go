package informer

import (
	"errors"
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/clock"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/net"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"github.com/tsundata/flowline/pkg/watch"
	"io"
	"math/rand"
	"reflect"
	"sync"
	"time"
)

var (
	// We try to spread the load on apiserver by setting timeouts for
	// watch requests - it is random in [minWatchTimeout, 2*minWatchTimeout].
	minWatchTimeout = 5 * time.Minute
)

// The WatchErrorHandler is called whenever ListAndWatch drops the
// connection with an error. After calling this handler, the informer
// will backoff and retry.
//
// The default implementation looks at the error type and tries to log
// the error message at an appropriate level.
//
// Implementations of this handler may display the error message in other
// ways. Implementations should return quickly - any expensive processing
// should be offloaded.
type WatchErrorHandler func(r *Reflector, err error)

// DefaultWatchErrorHandler is the default implementation of WatchErrorHandler
func DefaultWatchErrorHandler(r *Reflector, err error) {
	switch {
	case isExpiredError(err):
		// Don't set LastSyncResourceVersionUnavailable - LIST call with ResourceVersion=RV already
		// has a semantic that it returns data at least as fresh as provided RV.
		// So first try to LIST with setting RV to resource version of last observed object.
		flog.Infof("%s: watch of %v closed with: %v", r.name, r.expectedTypeName, err)
	case err == io.EOF:
		// watch closed normally
	case err == io.ErrUnexpectedEOF:
		flog.Infof("%s: Watch for %v closed with unexpected EOF: %v", r.name, r.expectedTypeName, err)
	default:
		flog.Errorf("%s: Failed to watch %v: %v", r.name, r.expectedTypeName, err)
	}
}

type Reflector struct {
	name             string
	expectedTypeName string
	expectedType     reflect.Type

	// The destination to sync up with the watch source
	store Store
	// listerWatcher is used to perform lists and watches.
	listerWatcher ListerWatcher

	// backoff manages backoff of ListWatch
	backoffManager parallelizer.BackoffManager
	// initConnBackoffManager manages backoff the initial connection with the Watch call of ListAndWatch.
	initConnBackoffManager parallelizer.BackoffManager

	resyncPeriod time.Duration
	// ShouldResync is invoked periodically and whenever it returns `true` the Store's Resync operation is invoked
	ShouldResync func() bool
	// clock allows tests to manipulate time
	clock clock.Clock

	// lastSyncResourceVersion is the resource version token last
	// observed when doing a sync with the underlying store
	// it is thread safe, but not synchronized with the underlying store
	lastSyncResourceVersion string
	// isLastSyncResourceVersionUnavailable is true if the previous list or watch request with
	// lastSyncResourceVersion failed with an "expired" or "too large resource version" error.
	isLastSyncResourceVersionUnavailable bool
	// lastSyncResourceVersionMutex guards read/write access to lastSyncResourceVersion
	lastSyncResourceVersionMutex sync.RWMutex

	// Called whenever the ListAndWatch drops the connection with an error.
	watchErrorHandler WatchErrorHandler
}

func NewReflector(lw ListerWatcher, expectedType interface{}, store Store, resyncPeriod time.Duration) *Reflector {
	return NewNamedReflector("reflector", lw, expectedType, store, resyncPeriod)
}

func NewNamedReflector(name string, lw ListerWatcher, expectedType interface{}, store Store, resyncPeriod time.Duration) *Reflector {
	realClock := &clock.RealClock{}
	r := &Reflector{
		name:          name,
		store:         store,
		listerWatcher: lw,
		// We used to make the call every 1sec (1 QPS), the goal here is to achieve ~98% traffic reduction when
		// API server is not healthy. With these parameters, backoff will stop at [30,60) sec interval which is
		// 0.22 QPS. If we don't backoff for 2min, assume API server is healthy and we reset the backoff.
		backoffManager:         parallelizer.NewExponentialBackoffManager(800*time.Millisecond, 30*time.Second, 2*time.Minute, 2.0, 1.0, realClock),
		initConnBackoffManager: parallelizer.NewExponentialBackoffManager(800*time.Millisecond, 30*time.Second, 2*time.Minute, 2.0, 1.0, realClock),
		resyncPeriod:           resyncPeriod,
		clock:                  realClock,
		watchErrorHandler:      WatchErrorHandler(DefaultWatchErrorHandler),
	}
	r.setExpectedType(expectedType)
	return r
}

const defaultExpectedTypeName = "<unspecified>"

func (r *Reflector) setExpectedType(expectedType interface{}) {
	r.expectedType = reflect.TypeOf(expectedType)
	if r.expectedType == nil {
		r.expectedTypeName = defaultExpectedTypeName
	}

	r.expectedTypeName = r.expectedType.String()
}

func (r *Reflector) Run(stopCh <-chan struct{}) {
	flog.Infof("starting reflector %s (%s) from %s", r.expectedTypeName, r.resyncPeriod, r.name)
	parallelizer.BackoffUntil(func() {
		if err := r.ListAndWatch(stopCh); err != nil {
			r.watchErrorHandler(r, err)
		}
	}, r.backoffManager, false, stopCh)
	flog.Infof("stopping reflector %s (%s) from %s", r.expectedTypeName, r.resyncPeriod, r.name)

}

func (r *Reflector) ListAndWatch(stopCh <-chan struct{}) error {
	flog.Infof("Listing and watching %v from %s", r.expectedTypeName, r.name)

	var resourceVersion string

	options := meta.ListOptions{ResourceVersion: r.relistResourceVersion()}

	if err := func() error {
		var list runtime.Object
		var err error

		listCh := make(chan struct{}, 1)
		panicCh := make(chan interface{}, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicCh <- r
				}

				list, err = r.listerWatcher.List(options)
				if err != nil {
					r.setIsLastSyncResourceVersionUnavailable(true)
					// todo retry
					list, err = r.listerWatcher.List(meta.ListOptions{ResourceVersion: r.relistResourceVersion()})
				}
				close(listCh)
			}()
		}()
		select {
		case <-stopCh:
			return nil
		case r := <-panicCh:
			panic(r)
		case <-listCh:
		}

		if err != nil {
			flog.Warnf("%s: failed to list %v: %v", r.name, r.expectedTypeName, err)
			return fmt.Errorf("failed to list %v: %w", r.expectedTypeName, err)
		}

		r.setIsLastSyncResourceVersionUnavailable(false)
		listMetaInterface, err := meta.ListAccessor(list)
		if err != nil {
			return fmt.Errorf("unable to understand list result %#v: %v", list, err)
		}
		resourceVersion = listMetaInterface.GetResourceVersion()
		items, err := meta.ExtractList(list)
		if err != nil {
			return fmt.Errorf("unable to understand list result %#v (%v)", list, err)
		}
		if err := r.syncWith(items, resourceVersion); err != nil {
			return fmt.Errorf("unable to sync list result: %v", err)
		}
		r.setLastSyncResourceVersion(resourceVersion)
		return nil
	}(); err != nil {
		return err
	}

	resyncerrc := make(chan error, 1)
	cancelCh := make(chan struct{})
	defer close(cancelCh)
	go func() {
		resyncCh, cleanup := r.resyncChan()
		defer func() {
			cleanup()
		}()
		for {
			select {
			case <-resyncCh:
			case <-stopCh:
				return
			case <-cancelCh:
				return
			}
			if r.ShouldResync == nil || r.ShouldResync() {
				flog.Infof("%s: forcing resync", r.name)
				if err := r.store.Resync(); err != nil {
					resyncerrc <- err
					return
				}
			}
			cleanup()
			resyncCh, cleanup = r.resyncChan()
		}
	}()

	for {
		select {
		case <-stopCh:
			return nil
		default:
		}

		timeoutSeconds := int64(minWatchTimeout.Seconds() * (rand.Float64() + 1.0))
		options = meta.ListOptions{
			ResourceVersion:     resourceVersion,
			TimeoutSeconds:      &timeoutSeconds,
			AllowWatchBookmarks: true,
		}

		start := r.clock.Now()
		w, err := r.listerWatcher.Watch(options)
		if err != nil {
			if net.IsConnectionRefused(err) || IsTooManyRequests(err) {
				<-r.initConnBackoffManager.Backoff().C()
			}
			return err
		}

		if err := r.watchHandler(start, w, &resourceVersion, resyncerrc, stopCh); err != nil {
			if err != errorStopRequested {
				switch {
				case isExpiredError(err):
					flog.Infof("%s: watch of %v closed with: %v", r.name, r.expectedTypeName, err)
				case IsTooManyRequests(err):
					flog.Infof("%s: watch of %v returned 429 - backing off", r.name, r.expectedTypeName)
					<-r.initConnBackoffManager.Backoff().C()
					continue
				default:
					flog.Warnf("%s: watch of %v ended with: %v", r.name, r.expectedTypeName, err)
				}
			}
			return nil
		}
	}
}

func IsTooManyRequests(_ error) bool {
	return false
}

func isExpiredError(_ error) bool {
	return false
}

func (r *Reflector) watchHandler(start time.Time, w watch.Interface, resourceVersion *string, errc chan error, stopCh <-chan struct{}) error {
	eventCount := 0

	defer w.Stop()

loop:
	for {
		select {
		case <-stopCh:
			return errorStopRequested
		case err := <-errc:
			return err
		case event, ok := <-w.ResultChan():
			if !ok {
				break loop
			}
			if event.Type == watch.Error {
				return fmt.Errorf("%s %+v", event.Type, event.Object)
			}
			if r.expectedType != nil {
				if e, a := r.expectedType, reflect.TypeOf(event.Object); e != a {
					flog.Errorf("%s: expected type %v, but watch event object had type %v", r.name, e, a)
					continue
				}
			}
			objMeta, err := meta.Accessor(event.Object)
			if err != nil {
				flog.Errorf("%s: unable to understand watch event %#v", r.name, event)
				continue
			}
			newResourceVersion := objMeta.GetResourceVersion()
			switch event.Type {
			case watch.Added:
				err := r.store.Add(event.Object)
				if err != nil {
					flog.Errorf("%s: unable to add watch event object (%#v) to store: %v", r.name, event.Object, err)
				}
			case watch.Modified:
				err := r.store.Update(event.Object)
				if err != nil {
					flog.Errorf("%s: unable to update watch event object (%#v) to store: %v", r.name, event.Object, err)
				}
			case watch.Deleted:
				err := r.store.Delete(event.Object)
				if err != nil {
					flog.Errorf("%s: unable to delete watch event object (%#v) to store: %v", r.name, event.Object, err)
				}
			case watch.Bookmark:
			default:
				flog.Errorf("%s: unable to understand watch event %#v", r.name, event)
			}
			*resourceVersion = newResourceVersion
			r.setLastSyncResourceVersion(newResourceVersion)
			if rvu, ok := r.store.(ResourceVersionUpdater); ok {
				rvu.UpdateResourceVersion(newResourceVersion)
			}
			eventCount++
		}
	}

	watchDuration := r.clock.Since(start)
	if watchDuration < 1*time.Second && eventCount == 0 {
		return fmt.Errorf("very short watch: %s: Unexpected watch close - watch lasted less than a second and no items received", r.name)
	}
	flog.Infof("%s: Watch close - %v total %v items received", r.name, r.expectedTypeName, eventCount)
	return nil
}

// ResourceVersionUpdater is an interface that allows store implementation to
// track the current resource version of the reflector. This is especially
// important if storage bookmarks are enabled.
type ResourceVersionUpdater interface {
	// UpdateResourceVersion is called each time current resource version of the reflector
	// is updated.
	UpdateResourceVersion(resourceVersion string)
}

func (r *Reflector) syncWith(items []runtime.Object, resourceVersion string) error {
	found := make([]interface{}, 0, len(items))
	for _, item := range items {
		found = append(found, item)
	}
	return r.store.Replace(found, resourceVersion)
}

var (
	// nothing will ever be sent down this channel
	neverExitWatch <-chan time.Time = make(chan time.Time)

	// Used to indicate that watching stopped because of a signal from the stop
	// channel passed in from a client of the reflector.
	errorStopRequested = errors.New("stop requested")
)

func (r *Reflector) resyncChan() (<-chan time.Time, func() bool) {
	if r.resyncPeriod == 0 {
		return neverExitWatch, func() bool {
			return false
		}
	}
	t := r.clock.NewTimer(r.resyncPeriod)
	return t.C(), t.Stop
}

func (r *Reflector) LastSyncResourceVersion() string {
	r.lastSyncResourceVersionMutex.RLock()
	defer r.lastSyncResourceVersionMutex.RUnlock()
	return r.lastSyncResourceVersion
}

func (r *Reflector) setLastSyncResourceVersion(v string) {
	r.lastSyncResourceVersionMutex.Lock()
	defer r.lastSyncResourceVersionMutex.Unlock()
	r.lastSyncResourceVersion = v
}

func (r *Reflector) setIsLastSyncResourceVersionUnavailable(isUnavailable bool) {
	r.lastSyncResourceVersionMutex.Lock()
	defer r.lastSyncResourceVersionMutex.Unlock()
	r.isLastSyncResourceVersionUnavailable = isUnavailable
}

func (r *Reflector) relistResourceVersion() string {
	r.lastSyncResourceVersionMutex.RLock()
	defer r.lastSyncResourceVersionMutex.RUnlock()

	if r.isLastSyncResourceVersionUnavailable {
		return ""
	}
	if r.lastSyncResourceVersion == "" {
		return "0"
	}
	return r.lastSyncResourceVersion
}
