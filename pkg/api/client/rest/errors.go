package rest

import (
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
)

// ErrorReporter converts generic errors into runtime.Object errors without
// requiring the caller to take a dependency on meta/v1 (where Status lives).
// This prevents circular dependencies in core watch code.
type ErrorReporter struct {
	code   int
	verb   string
	reason string
}

// NewClientErrorReporter will respond with valid v1.Status objects that report
// unexpected server responses. Primarily used by watch to report errors when
// we attempt to decode a response from the server and it is not in the form
// we expect. Because watch is a dependency of the core api, we can't return
// meta/v1.Status in that package and so much inject this interface to convert a
// generic error as appropriate. The reason is passed as a unique status cause
// on the returned status, otherwise the generic "ClientError" is returned.
func NewClientErrorReporter(code int, verb string, reason string) *ErrorReporter {
	return &ErrorReporter{
		code:   code,
		verb:   verb,
		reason: reason,
	}
}

// AsObject returns a valid error runtime.Object (a v1.Status) for the given
// error, using the code and verb of the reporter type. The error is set to
// indicate that this was an unexpected server response.
func (r *ErrorReporter) AsObject(err error) runtime.Object {
	return &meta.Status{
		Status: "Failure",
		Reason: r.reason,
		Code:   int32(r.code),
	}
}
