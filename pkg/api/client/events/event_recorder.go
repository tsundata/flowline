package events

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/client/record/util"
	"github.com/tsundata/flowline/pkg/api/client/reference"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/util/clock"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"github.com/tsundata/flowline/pkg/watch"
	"time"
)

type recorderImpl struct {
	scheme              *runtime.Scheme
	source              meta.EventSource
	reportingController string
	reportingInstance   string
	*watch.Broadcaster
	clock clock.Clock
}

func (recorder *recorderImpl) Eventf(regarding runtime.Object, related runtime.Object, eventtype, reason, action, messageFmt string, args ...interface{}) {
	timestamp := time.Now()
	message := fmt.Sprintf(messageFmt, args...)
	refRegarding, err := reference.GetReference(recorder.scheme, regarding)
	if err != nil {
		flog.Errorf("Could not construct reference to: '%#v' due to: '%v'. Will not report event: '%v' '%v' '%v'", regarding, err, eventtype, reason, message)
		return
	}

	var refRelated *meta.ObjectReference
	if related != nil {
		refRelated, err = reference.GetReference(recorder.scheme, related)
		if err != nil {
			flog.Infof("Could not construct reference to: '%#v' due to: '%v'.", related, err)
		}
	}
	if !util.ValidateEventType(eventtype) {
		flog.Errorf("Unsupported event type: '%v'", eventtype)
		return
	}
	event := recorder.makeEvent(refRegarding, refRelated, &timestamp, eventtype, reason, message, recorder.reportingController, recorder.reportingInstance, action)
	event.Source = recorder.source
	go func() {
		defer parallelizer.HandleCrash()
		_ = recorder.Action(watch.Added, event)
	}()
}

func (recorder *recorderImpl) makeEvent(refRegarding *meta.ObjectReference, refRelated *meta.ObjectReference, timestamp *time.Time, eventtype, reason, message string, reportingController string, reportingInstance string, action string) *meta.Event {
	t := recorder.clock.Now()
	return &meta.Event{
		ObjectMeta: meta.ObjectMeta{
			Name: fmt.Sprintf("%v.%x", refRegarding.UID, t.UnixNano()),
		},
		Series:              nil,
		ReportingController: reportingController,
		ReportingInstance:   reportingInstance,
		Action:              action,
		Reason:              reason,
		Regarding:           *refRegarding,
		Related:             refRelated,
		Message:             message,
		Type:                eventtype,
		Count:               1,
		FirstTimestamp:      t,
		LastTimestamp:       &t,
		EventTime:           timestamp,
	}
}
