package runtime

import (
	"github.com/tsundata/flowline/pkg/util/flog"
	"sync"
	"time"
)

const (
	ContentTypeJSON string = "application/json"
)

// HandleError is a method to invoke when a non-user facing piece of code cannot
// return an error and needs to indicate it has been ignored. Invoking this method
// is preferable to logging the error - the default behavior is to log but the
// errors may be sent to a remote server for analysis.
func HandleError(err error) {
	// this is sometimes called with a nil error.  We probably shouldn't fail and should do nothing instead
	if err == nil {
		return
	}

	for _, fn := range ErrorHandlers {
		fn(err)
	}
}

// ErrorHandlers is a list of functions which will be invoked when a nonreturnable
// error occurs.
var ErrorHandlers = []func(error){
	logError,
	(&rudimentaryErrorBackoff{
		lastErrorTime: time.Now(),
		// 1ms was the number folks were able to stomach as a global rate limit.
		// If you need to log errors more than 1000 times a second you
		// should probably consider fixing your code instead. :)
		minPeriod: time.Millisecond,
	}).OnError,
}

// logError prints an error with the call stack of the location it was reported
func logError(err error) {
	flog.Error(err)
}

type rudimentaryErrorBackoff struct {
	minPeriod         time.Duration // immutable
	lastErrorTimeLock sync.Mutex
	lastErrorTime     time.Time
}

// OnError will block if it is called more often than the embedded period time.
// This will prevent overly tight hot error loops.
func (r *rudimentaryErrorBackoff) OnError(error) {
	r.lastErrorTimeLock.Lock()
	defer r.lastErrorTimeLock.Unlock()
	d := time.Since(r.lastErrorTime)
	if d < r.minPeriod {
		// If the time moves backwards for any reason, do nothing
		time.Sleep(r.minPeriod - d)
	}
	r.lastErrorTime = time.Now()
}
