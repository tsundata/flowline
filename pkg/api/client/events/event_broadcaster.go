package events

import (
	"context"
	"encoding/json"
	"github.com/tsundata/flowline/pkg/api/client"
	eventsv1 "github.com/tsundata/flowline/pkg/api/client/events/v1"
	"github.com/tsundata/flowline/pkg/api/client/record"
	"github.com/tsundata/flowline/pkg/api/client/record/util"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/clock"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"github.com/tsundata/flowline/pkg/watch"
	"os"
	"sync"
	"time"
)

const (
	maxTriesPerEvent = 12
	finishTime       = 6 * time.Minute
	refreshTime      = 30 * time.Minute
	maxQueuedEvents  = 1000
)

var defaultSleepDuration = 10 * time.Second

type eventKey struct {
	action              string
	reason              string
	reportingController string
	regarding           meta.ObjectReference
	related             meta.ObjectReference
}

type eventBroadcasterImpl struct {
	*watch.Broadcaster
	mu            sync.Mutex
	eventCache    map[eventKey]*meta.Event
	sleepDuration time.Duration
	sink          EventSink
}

// EventSinkImpl wraps EventsV1Interface to implement EventSink.
type EventSinkImpl struct {
	Interface eventsv1.EventsV1Interface
}

// Create takes the representation of an event and creates it. Returns the server's representation of the event, and an error, if there is any.
func (e *EventSinkImpl) Create(event *meta.Event) (*meta.Event, error) {
	return e.Interface.Event().Create(context.TODO(), event, meta.CreateOptions{})
}

// Update takes the representation of an event and updates it. Returns the server's representation of the event, and an error, if there is any.
func (e *EventSinkImpl) Update(event *meta.Event) (*meta.Event, error) {
	return e.Interface.Event().Update(context.TODO(), event, meta.UpdateOptions{})
}

// Patch applies the patch and returns the patched event, and an error, if there is any.
func (e *EventSinkImpl) Patch(event *meta.Event, data []byte) (*meta.Event, error) {
	return e.Interface.Event().Patch(context.TODO(), event.Name, string(meta.MergePatchType), data, meta.PatchOptions{})
}

// NewBroadcaster Creates a new event broadcaster.
func NewBroadcaster(sink EventSink) EventBroadcaster {
	return newBroadcaster(sink, defaultSleepDuration, map[eventKey]*meta.Event{})
}

// NewBroadcasterForTest Creates a new event broadcaster for test purposes.
func newBroadcaster(sink EventSink, sleepDuration time.Duration, eventCache map[eventKey]*meta.Event) EventBroadcaster {
	return &eventBroadcasterImpl{
		Broadcaster:   watch.NewBroadcaster(maxQueuedEvents, watch.DropIfChannelFull),
		eventCache:    eventCache,
		sleepDuration: sleepDuration,
		sink:          sink,
	}
}

func (e *eventBroadcasterImpl) Shutdown() {
	e.Broadcaster.Shutdown()
}

// refreshExistingEventSeries refresh events TTL
func (e *eventBroadcasterImpl) refreshExistingEventSeries() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for isomorphicKey, event := range e.eventCache {
		if event.Series != nil {
			if recordedEvent, retry := recordEvent(e.sink, event); !retry {
				if recordedEvent != nil {
					e.eventCache[isomorphicKey] = recordedEvent
				}
			}
		}
	}
}

// finishSeries checks if a series has ended and either:
// - write final count to the apiserver
// - delete a singleton event (i.e. series field is nil) from the cache
func (e *eventBroadcasterImpl) finishSeries() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for isomorphicKey, event := range e.eventCache {
		eventSerie := event.Series
		if eventSerie != nil {
			if eventSerie.LastObservedTime.Before(time.Now().Add(-finishTime)) {
				if _, retry := recordEvent(e.sink, event); !retry {
					delete(e.eventCache, isomorphicKey)
				}
			}
		} else if event.EventTime.Before(time.Now().Add(-finishTime)) {
			delete(e.eventCache, isomorphicKey)
		}
	}
}

// NewRecorder returns an EventRecorder that records events with the given event source.
func (e *eventBroadcasterImpl) NewRecorder(scheme *runtime.Scheme, reportingController string) EventRecorder {
	hostname, _ := os.Hostname()
	reportingInstance := reportingController + "-" + hostname
	source := meta.EventSource{Component: reportingController}
	return &recorderImpl{scheme, source, reportingController, reportingInstance, e.Broadcaster, clock.RealClock{}}
}

func (e *eventBroadcasterImpl) recordToSink(event *meta.Event, clock clock.Clock) {
	// Make a copy before modification, because there could be multiple listeners.
	eventCopy := event
	go func() {
		evToRecord := func() *meta.Event {
			e.mu.Lock()
			defer e.mu.Unlock()
			now := clock.Now()
			eventKey := getKey(eventCopy)
			isomorphicEvent, isIsomorphic := e.eventCache[eventKey]
			if isIsomorphic {
				if isomorphicEvent.Series != nil {
					isomorphicEvent.Series.Count++
					isomorphicEvent.Series.LastObservedTime = &now
					return nil
				}
				isomorphicEvent.Series = &meta.EventSeries{
					Count:            1,
					LastObservedTime: &now,
				}
				return isomorphicEvent
			}
			e.eventCache[eventKey] = eventCopy
			return eventCopy
		}()
		if evToRecord != nil {
			recordedEvent := e.attemptRecording(evToRecord)
			if recordedEvent != nil {
				recordedEventKey := getKey(recordedEvent)
				e.mu.Lock()
				defer e.mu.Unlock()
				e.eventCache[recordedEventKey] = recordedEvent
			}
		}
	}()
}

func (e *eventBroadcasterImpl) attemptRecording(event *meta.Event) *meta.Event {
	tries := 0
	for {
		if recordedEvent, retry := recordEvent(e.sink, event); !retry {
			return recordedEvent
		}
		tries++
		if tries >= maxTriesPerEvent {
			flog.Errorf("Unable to write event '%#v' (retry limit exceeded!)", event)
			return nil
		}
		// Randomize sleep so that various clients won't all be
		// synced up if the master goes down.
		time.Sleep(parallelizer.Jitter(e.sleepDuration, 0.25))
	}
}

func recordEvent(sink EventSink, event *meta.Event) (*meta.Event, bool) {
	var newEvent *meta.Event
	var err error
	isEventSeries := event.Series != nil
	if isEventSeries {
		patch, patchBytesErr := createPatchBytesForSeries(event)
		if patchBytesErr != nil {
			flog.Errorf("Unable to calculate diff, no merge is possible: %v", patchBytesErr)
			return nil, false
		}
		newEvent, err = sink.Patch(event, patch)
	}
	// Update can fail because the event may have been removed and it no longer exists.
	if !isEventSeries || (isEventSeries && util.IsKeyNotFoundError(err)) {
		// Making sure that ResourceVersion is empty on creation
		event.ResourceVersion = ""
		newEvent, err = sink.Create(event)
	}
	if err == nil {
		return newEvent, false
	}
	// If we can't contact the server, then hold everything while we keep trying.
	// Otherwise, something about the event is malformed, and we should abandon it.
	flog.Errorf("Unable to write event: '%v' (may retry after sleeping)", err)
	return nil, true
}

func createPatchBytesForSeries(event *meta.Event) ([]byte, error) {
	//oldEvent := event.DeepCopyObject() todo
	oldEvent := event
	oldEvent.Series = nil
	oldData, err := json.Marshal(oldEvent)
	if err != nil {
		return nil, err
	}
	newData, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	return util.CreateTwoWayMergePatch(oldData, newData, meta.Event{})
}

func getKey(event *meta.Event) eventKey {
	key := eventKey{
		action:              event.Action,
		reason:              event.Reason,
		reportingController: event.ReportingController,
		regarding:           event.Regarding,
	}
	if event.Related != nil {
		key.related = *event.Related
	}
	return key
}

// StartStructuredLogging starts sending events received from this EventBroadcaster to the structured logging function.
// The return value can be ignored or used to stop recording, if desired.
func (e *eventBroadcasterImpl) StartStructuredLogging(_ string) func() {
	return e.StartEventWatcher(
		func(obj runtime.Object) {
			event, ok := obj.(*meta.Event)
			if !ok {
				flog.Errorf("unexpected type, expected meta.Event")
				return
			}
			flog.Infof("Event occurred %s %s %s %s %s %s %s", event.Regarding.UID, event.Regarding.Kind, event.Regarding.APIVersion, event.Type, event.Reason, event.Action, event.Note)
		})
}

// StartEventWatcher starts sending events received from this EventBroadcaster to the given event handler function.
// The return value is used to stop recording
func (e *eventBroadcasterImpl) StartEventWatcher(eventHandler func(event runtime.Object)) func() {
	watcher, err := e.Watch()
	if err != nil {
		flog.Errorf("Unable start event watcher: '%v' (will not retry!)", err)
		return func() {
			flog.Errorf("The event watcher failed to start")
		}
	}
	go func() {
		defer parallelizer.HandleCrash()
		for {
			watchEvent, ok := <-watcher.ResultChan()
			if !ok {
				return
			}
			eventHandler(watchEvent.Object)
		}
	}()
	return watcher.Stop
}

func (e *eventBroadcasterImpl) startRecordingEvents(stopCh <-chan struct{}) {
	eventHandler := func(obj runtime.Object) {
		event, ok := obj.(*meta.Event)
		if !ok {
			flog.Errorf("unexpected type, expected meta.Event")
			return
		}
		e.recordToSink(event, clock.RealClock{})
	}
	stopWatcher := e.StartEventWatcher(eventHandler)
	go func() {
		<-stopCh
		stopWatcher()
	}()
}

// StartRecordingToSink starts sending events received from the specified eventBroadcaster to the given sink.
func (e *eventBroadcasterImpl) StartRecordingToSink(stopCh <-chan struct{}) {
	go parallelizer.Until(e.refreshExistingEventSeries, refreshTime, stopCh)
	go parallelizer.Until(e.finishSeries, finishTime, stopCh)
	e.startRecordingEvents(stopCh)
}

type eventBroadcasterAdapterImpl struct {
	coreClient          eventsv1.EventGetter    // Deprecated: use eventsv1Client
	coreBroadcaster     record.EventBroadcaster // Deprecated: use eventsv1Broadcaster
	eventsv1Client      eventsv1.EventsV1Interface
	eventsv1Broadcaster EventBroadcaster
}

// NewEventBroadcasterAdapter creates a wrapper around new and legacy broadcasters to simplify
// migration of individual components to the new Event API.
func NewEventBroadcasterAdapter(client client.Interface) EventBroadcasterAdapter {
	eventClient := &eventBroadcasterAdapterImpl{}
	eventClient.eventsv1Client = client.EventsV1()
	eventClient.eventsv1Broadcaster = NewBroadcaster(&EventSinkImpl{Interface: eventClient.eventsv1Client})

	// Even though there can soon exist cases when coreBroadcaster won't really be needed,
	// we create it unconditionally because its overhead is minor and will simplify using usage
	// patterns of this library in all components.
	eventClient.coreClient = client.EventsV1()
	eventClient.coreBroadcaster = record.NewBroadcaster()
	return eventClient
}

// StartRecordingToSink starts sending events received from the specified eventBroadcaster to the given sink.
func (e *eventBroadcasterAdapterImpl) StartRecordingToSink(stopCh <-chan struct{}) {
	if e.eventsv1Broadcaster != nil && e.eventsv1Client != nil {
		e.eventsv1Broadcaster.StartRecordingToSink(stopCh)
	}
}

func (e *eventBroadcasterAdapterImpl) NewRecorder(name string) EventRecorder {
	if e.eventsv1Broadcaster != nil && e.eventsv1Client != nil {
		return e.eventsv1Broadcaster.NewRecorder(runtime.NewScheme(), name)
	}
	return record.NewEventRecorderAdapter(e.DeprecatedNewLegacyRecorder(name))
}

func (e *eventBroadcasterAdapterImpl) DeprecatedNewLegacyRecorder(name string) record.EventRecorder {
	return e.coreBroadcaster.NewRecorder(runtime.NewScheme(), meta.EventSource{Component: name})
}

func (e *eventBroadcasterAdapterImpl) Shutdown() {
	if e.coreBroadcaster != nil {
		e.coreBroadcaster.Shutdown()
	}
	if e.eventsv1Broadcaster != nil {
		e.eventsv1Broadcaster.Shutdown()
	}
}
