package record

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/client/record/util"
	"github.com/tsundata/flowline/pkg/api/client/reference"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/clock"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"github.com/tsundata/flowline/pkg/util/uid"
	"github.com/tsundata/flowline/pkg/watch"
	"math/rand"
	"time"
)

const maxTriesPerEvent = 12

var defaultSleepDuration = 10 * time.Second

const maxQueuedEvents = 1000

// EventSink knows how to store events (client.Client implements it.)
// EventSink must respect the namespace that will be embedded in 'event'.
// It is assumed that EventSink will return the same sorts of errors as
// pkg/client's REST client.
type EventSink interface {
	Create(event *meta.Event) (*meta.Event, error)
	Update(event *meta.Event) (*meta.Event, error)
	Patch(oldEvent *meta.Event, data []byte) (*meta.Event, error)
}

// CorrelatorOptions allows you to change the default of the EventSourceObjectSpamFilter
// and EventAggregator in EventCorrelator
type CorrelatorOptions struct {
	// The lru cache size used for both EventSourceObjectSpamFilter and the EventAggregator
	// If not specified (zero value), the default specified in events_cache.go will be picked
	// This means that the LRUCacheSize has to be greater than 0.
	LRUCacheSize int
	// The burst size used by the token bucket rate filtering in EventSourceObjectSpamFilter
	// If not specified (zero value), the default specified in events_cache.go will be picked
	// This means that the BurstSize has to be greater than 0.
	BurstSize int
	// The fill rate of the token bucket in queries per second in EventSourceObjectSpamFilter
	// If not specified (zero value), the default specified in events_cache.go will be picked
	// This means that the QPS has to be greater than 0.
	QPS float32
	// The func used by the EventAggregator to group event keys for aggregation
	// If not specified (zero value), EventAggregatorByReasonFunc will be used
	KeyFunc EventAggregatorKeyFunc
	// The func used by the EventAggregator to produced aggregated message
	// If not specified (zero value), EventAggregatorByReasonMessageFunc will be used
	MessageFunc EventAggregatorMessageFunc
	// The number of events in an interval before aggregation happens by the EventAggregator
	// If not specified (zero value), the default specified in events_cache.go will be picked
	// This means that the MaxEvents has to be greater than 0
	MaxEvents int
	// The amount of time in seconds that must transpire since the last occurrence of a similar event before it is considered new by the EventAggregator
	// If not specified (zero value), the default specified in events_cache.go will be picked
	// This means that the MaxIntervalInSeconds has to be greater than 0
	MaxIntervalInSeconds int
	// The clock used by the EventAggregator to allow for testing
	// If not specified (zero value), clock.RealClock{} will be used
	Clock clock.PassiveClock
	// The func used by EventFilterFunc, which returns a key for given event, based on which filtering will take place
	// If not specified (zero value), getSpamKey will be used
	SpamKeyFunc EventSpamKeyFunc
}

// EventRecorder knows how to record events on behalf of an EventSource.
type EventRecorder interface {
	// Event constructs an event from the given information and puts it in the queue for sending.
	// 'object' is the object this event is about. Event will make a reference-- or you may also
	// pass a reference to the object directly.
	// 'eventtype' of this event, and can be one of Normal, Warning. New types could be added in future
	// 'reason' is the reason this event is generated. 'reason' should be short and unique; it
	// should be in UpperCamelCase format (starting with a capital letter). "reason" will be used
	// to automate handling of events, so imagine people writing switch statements to handle them.
	// You want to make that easy.
	// 'message' is intended to be human-readable.
	//
	// The resulting event will be created in the same namespace as the reference object.
	Event(object runtime.Object, eventtype, reason, message string)

	// Eventf is just like Event, but with Sprintf for the message field.
	Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{})

	// AnnotatedEventf is just like eventf, but with annotations attached fixme delete
	AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{})
}

// EventBroadcaster knows how to receive events and send them to any EventSink, watcher, or log.
type EventBroadcaster interface {
	// StartEventWatcher starts sending events received from this EventBroadcaster to the given
	// event handler function. The return value can be ignored or used to stop recording, if
	// desired.
	StartEventWatcher(eventHandler func(*meta.Event)) watch.Interface

	// StartRecordingToSink starts sending events received from this EventBroadcaster to the given
	// sink. The return value can be ignored or used to stop recording, if desired.
	StartRecordingToSink(sink EventSink) watch.Interface

	// StartLogging starts sending events received from this EventBroadcaster to the given logging
	// function. The return value can be ignored or used to stop recording, if desired.
	StartLogging(logf func(format string, args ...interface{})) watch.Interface

	// StartStructuredLogging starts sending events received from this EventBroadcaster to the structured
	// logging function. The return value can be ignored or used to stop recording, if desired.
	StartStructuredLogging(verbosity string) watch.Interface

	// NewRecorder returns an EventRecorder that can be used to send events to this EventBroadcaster
	// with the event source set to the given event source.
	NewRecorder(scheme *runtime.Scheme, source meta.EventSource) EventRecorder

	// Shutdown shuts down the broadcaster
	Shutdown()
}

// EventRecorderAdapter is a wrapper around a EventRecorder
type EventRecorderAdapter struct {
	recorder EventRecorder
}

func NewEventRecorderAdapter(recorder EventRecorder) *EventRecorderAdapter {
	return &EventRecorderAdapter{recorder: recorder}
}

func (a *EventRecorderAdapter) Eventf(regarding runtime.Object, _ runtime.Object, eventtype, reason, _, note string, args ...interface{}) {
	a.recorder.Eventf(regarding, eventtype, reason, note, args...)
}

type eventBroadcasterImpl struct {
	*watch.Broadcaster
	sleepDuration time.Duration
	options       CorrelatorOptions
}

// NewBroadcaster Creates a new event broadcaster.
func NewBroadcaster() EventBroadcaster {
	return &eventBroadcasterImpl{
		Broadcaster:   watch.NewLongQueueBroadcaster(maxQueuedEvents, watch.DropIfChannelFull),
		sleepDuration: defaultSleepDuration,
	}
}

// StartRecordingToSink starts sending events received from the specified eventBroadcaster to the given sink.
// The return value can be ignored or used to stop recording, if desired.
func (e *eventBroadcasterImpl) StartRecordingToSink(sink EventSink) watch.Interface {
	eventCorrelator := NewEventCorrelatorWithOptions(e.options)
	return e.StartEventWatcher(
		func(event *meta.Event) {
			recordToSink(sink, event, eventCorrelator, e.sleepDuration)
		})
}

func (e *eventBroadcasterImpl) Shutdown() {
	e.Broadcaster.Shutdown()
}

func recordToSink(sink EventSink, event *meta.Event, eventCorrelator *EventCorrelator, sleepDuration time.Duration) {
	// Make a copy before modification, because there could be multiple listeners.
	// Events are safe to copy like this.
	eventCopy := *event
	event = &eventCopy
	result, err := eventCorrelator.EventCorrelate(event)
	if err != nil {
		runtime.HandleError(err)
	}
	if result.Skip {
		return
	}
	tries := 0
	for {
		if recordEvent(sink, result.Event, result.Patch, result.Event.Count > 1, eventCorrelator) {
			break
		}
		tries++
		if tries >= maxTriesPerEvent {
			flog.Errorf("Unable to write event '%#v' (retry limit exceeded!)", event)
			break
		}
		// Randomize the first sleep so that various clients won't all be
		// synced up if the master goes down.
		if tries == 1 {
			time.Sleep(time.Duration(float64(sleepDuration) * rand.Float64()))
		} else {
			time.Sleep(sleepDuration)
		}
	}
}

// recordEvent attempts to write event to a sink. It returns true if the event
// was successfully recorded or discarded, false if it should be retried.
// If updateExistingEvent is false, it creates a new event, otherwise it updates
// existing event.
func recordEvent(sink EventSink, event *meta.Event, patch []byte, updateExistingEvent bool, eventCorrelator *EventCorrelator) bool {
	var newEvent *meta.Event
	var err error
	if updateExistingEvent {
		newEvent, err = sink.Patch(event, patch)
	}
	// Update can fail because the event may have been removed and it no longer exists.
	if !updateExistingEvent || (updateExistingEvent && util.IsKeyNotFoundError(err)) {
		// Making sure that ResourceVersion is empty on creation
		event.ResourceVersion = ""
		newEvent, err = sink.Create(event)
	}
	if err == nil {
		// we need to update our event correlator with the server returned state to handle name/resourceversion
		eventCorrelator.UpdateState(newEvent)
		return true
	}

	// If we can't contact the server, then hold everything while we keep trying.
	// Otherwise, something about the event is malformed and we should abandon it.
	flog.Errorf("Unable to write event: '%#v': '%v'(may retry after sleeping)", event, err)
	return false
}

// StartLogging starts sending events received from this EventBroadcaster to the given logging function.
// The return value can be ignored or used to stop recording, if desired.
func (e *eventBroadcasterImpl) StartLogging(logf func(format string, args ...interface{})) watch.Interface {
	return e.StartEventWatcher(
		func(e *meta.Event) {
			logf("Event(%#v): type: '%v' reason: '%v' %v", e.InvolvedObject, e.Type, e.Reason, e.Message)
		})
}

// StartStructuredLogging starts sending events received from this EventBroadcaster to the structured logging function.
// The return value can be ignored or used to stop recording, if desired.
func (e *eventBroadcasterImpl) StartStructuredLogging(_ string) watch.Interface {
	return e.StartEventWatcher(
		func(e *meta.Event) {
			flog.Infof("Event occurred %s %s %s %s %s %s %s", e.InvolvedObject.Name, e.InvolvedObject.FieldPath, e.InvolvedObject.Kind, e.InvolvedObject.APIVersion, e.Type, e.Reason, e.Message)
		})
}

// StartEventWatcher starts sending events received from this EventBroadcaster to the given event handler function.
// The return value can be ignored or used to stop recording, if desired.
func (e *eventBroadcasterImpl) StartEventWatcher(eventHandler func(*meta.Event)) watch.Interface {
	watcher, err := e.Watch()
	if err != nil {
		flog.Errorf("Unable start event watcher: '%v' (will not retry!)", err)
	}
	go func() {
		defer parallelizer.HandleCrash()
		for watchEvent := range watcher.ResultChan() {
			event, ok := watchEvent.Object.(*meta.Event)
			if !ok {
				// This is all local, so there's no reason this should
				// ever happen.
				continue
			}
			eventHandler(event)
		}
	}()
	return watcher
}

// NewRecorder returns an EventRecorder that records events with the given event source.
func (e *eventBroadcasterImpl) NewRecorder(scheme *runtime.Scheme, source meta.EventSource) EventRecorder {
	return &recorderImpl{scheme, source, e.Broadcaster, clock.RealClock{}}
}

type recorderImpl struct {
	scheme *runtime.Scheme
	source meta.EventSource
	*watch.Broadcaster
	clock clock.PassiveClock
}

func (recorder *recorderImpl) generateEvent(object runtime.Object, eventtype, reason, message string) {
	ref, err := reference.GetReference(recorder.scheme, object)
	if err != nil {
		flog.Errorf("Could not construct reference to: '%#v' due to: '%v'. Will not report event: '%v' '%v' '%v'", object, err, eventtype, reason, message)
		return
	}

	if !util.ValidateEventType(eventtype) {
		flog.Errorf("Unsupported event type: '%v'", eventtype)
		return
	}

	event := recorder.makeEvent(ref, eventtype, reason, message)
	event.Source = recorder.source

	// NOTE: events should be a non-blocking operation, but we also need to not
	// put this in a goroutine, otherwise we'll race to write to a closed channel
	// when we go to shut down this broadcaster.  Just drop events if we get overloaded,
	// and log an error if that happens (we've configured the broadcaster to drop
	// outgoing events anyway).
	sent, err := recorder.ActionOrDrop(watch.Added, event)
	if err != nil {
		flog.Errorf("unable to record event: %v (will not retry!)", err)
		return
	}
	if !sent {
		flog.Errorf("unable to record event: too many queued events, dropped event %#v", event)
	}
}

func (recorder *recorderImpl) Event(object runtime.Object, eventtype, reason, message string) {
	recorder.generateEvent(object, eventtype, reason, message)
}

func (recorder *recorderImpl) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	recorder.Event(object, eventtype, reason, fmt.Sprintf(messageFmt, args...))
}

func (recorder *recorderImpl) AnnotatedEventf(object runtime.Object, _ map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	recorder.generateEvent(object, eventtype, reason, fmt.Sprintf(messageFmt, args...))
}

func (recorder *recorderImpl) makeEvent(ref *meta.ObjectReference, eventtype, reason, message string) *meta.Event {
	t := recorder.clock.Now()
	return &meta.Event{
		ObjectMeta: meta.ObjectMeta{
			UID:  uid.New(),
			Name: fmt.Sprintf("%v.%x", ref.Name, t.UnixNano()),
		},
		InvolvedObject: *ref,
		Reason:         reason,
		Message:        message,
		FirstTimestamp: t,
		LastTimestamp:  &t,
		Count:          1,
		Type:           eventtype,
	}
}
