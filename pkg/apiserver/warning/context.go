package warning

import "context"

// The key type is unexported to prevent collisions
type key int

const (
	// auditAnnotationsKey is the context key for the audit annotations.
	warningRecorderKey key = iota
)

// Recorder provides a method for recording warnings
type Recorder interface {
	// AddWarning adds the specified warning to the response.
	// agent must be valid UTF-8, and must not contain spaces, quotes, backslashes, or control characters.
	// text must be valid UTF-8, and must not contain control characters.
	AddWarning(agent, text string)
}

// WithWarningRecorder returns a new context that wraps the provided context and contains the provided Recorder implementation.
// The returned context can be passed to AddWarning().
func WithWarningRecorder(ctx context.Context, recorder Recorder) context.Context {
	return context.WithValue(ctx, warningRecorderKey, recorder)
}
func warningRecorderFrom(ctx context.Context) (Recorder, bool) {
	recorder, ok := ctx.Value(warningRecorderKey).(Recorder)
	return recorder, ok
}

// AddWarning records a warning for the specified agent and text to the Recorder added to the provided context using WithWarningRecorder().
// If no Recorder exists in the provided context, this is a no-op.
// agent must be valid UTF-8, and must not contain spaces, quotes, backslashes, or control characters.
// text must be valid UTF-8, and must not contain control characters.
func AddWarning(ctx context.Context, agent string, text string) {
	recorder, ok := warningRecorderFrom(ctx)
	if !ok {
		return
	}
	recorder.AddWarning(agent, text)
}
