package registry

import (
	"bytes"
	"context"
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry/negotiation"
	"github.com/tsundata/flowline/pkg/apiserver/storage"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"github.com/tsundata/flowline/pkg/runtime/serializer/json"
	"github.com/tsundata/flowline/pkg/runtime/serializer/streaming"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/parallelizer"
	"github.com/tsundata/flowline/pkg/watch"
	"golang.org/x/net/websocket"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func (e *Store) GetHandler(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	uid := req.PathParameter("uid")
	out, err := e.Get(ctx, uid, &storage.GetOptions{}) // todo resourceVersion
	if err != nil {
		_ = resp.WriteError(http.StatusNotFound, err)
		return
	}
	_ = resp.WriteEntity(out)
}

func (e *Store) CreateHandler(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	obj := e.New()
	err := req.ReadEntity(&obj)
	if err != nil {
		_ = resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	out, err := e.Create(ctx, obj, nil, nil)
	if err != nil {
		_ = resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	_ = resp.WriteEntity(out)
}

func (e *Store) UpdateHandler(req *restful.Request, resp *restful.Response) {
	uid := req.PathParameter("uid")
	ctx := req.Request.Context()
	obj := e.New()
	err := req.ReadEntity(&obj)
	if err != nil {
		_ = resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	out, _, err := e.Update(ctx, uid, obj, nil, nil, true, nil)
	if err != nil {
		_ = resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	_ = resp.WriteEntity(out)
}

func (e *Store) DeleteHandler(req *restful.Request, resp *restful.Response) {
	uid := req.PathParameter("uid")
	ctx := req.Request.Context()
	out, _, err := e.Delete(ctx, uid, nil, nil)
	if err != nil {
		_ = resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	_ = resp.WriteEntity(out)
}

func (e *Store) ListHandler(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	out, err := e.List(ctx, &storage.ListOptions{})
	if err != nil {
		_ = resp.WriteError(http.StatusInternalServerError, err)
		return
	}
	_ = resp.WriteEntity(out)
}

const timeout = 30 * 60 * time.Second

func (e *Store) WatchHandler(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	uid := req.PathParameter("uid")

	scope := newRequestScope()

	outputMediaType, _, err := negotiation.NegotiateOutputMediaType(req.Request, scope.Serializer, scope)
	if err != nil {
		_ = resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	watcher, err := e.Watch(ctx, &storage.ListOptions{Field: uid, Recursive: false})
	if err != nil {
		_ = resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	serveWatch(watcher, scope, outputMediaType, req.Request, resp, timeout)
}

func (e *Store) WatchListHandler(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()

	scope := newRequestScope()

	outputMediaType, _, err := negotiation.NegotiateOutputMediaType(req.Request, scope.Serializer, scope)
	if err != nil {
		_ = resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	watcher, err := e.Watch(ctx, &storage.ListOptions{Recursive: true})
	if err != nil {
		_ = resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	serveWatch(watcher, scope, outputMediaType, req.Request, resp, timeout)
}

func serveWatch(watcher watch.Interface, scope *RequestScope, mediaTypeOptions negotiation.MediaTypeOptions, req *http.Request, w http.ResponseWriter, timeout time.Duration) {
	defer watcher.Stop()

	serializer, err := negotiation.NegotiateOutputMediaTypeStream(req, scope.Serializer, scope)
	if err != nil {
		flog.Error(err)
		return
	}
	framer := serializer.StreamSerializer.Framer
	streamSerializer := serializer.StreamSerializer.Serializer
	encoder := scope.Serializer.EncoderForVersion(streamSerializer, scope.Kind.GroupVersion())
	if framer == nil {
		err = fmt.Errorf("no framer defined for %q available for embedded encoding", serializer.MediaType)
		flog.Error(err)
		return
	}
	mediaType := serializer.MediaType
	if mediaType != "application/json" {
		mediaType += ";stream=watch"
	}

	var embeddedEncoder runtime.Encoder
	jsonCoder := runtime.JsonCoder{}
	codec := runtime.NewBase64Serializer(jsonCoder, jsonCoder)
	embeddedEncoder = codec

	server := &WatchServer{
		Watching: watcher,
		Scope:    scope,

		MediaType:       mediaType,
		Framer:          framer,
		Encoder:         encoder,
		EmbeddedEncoder: embeddedEncoder,

		Fixup: func(object runtime.Object) runtime.Object {
			return object
		},
		TimeoutFactory: &realTimeoutFactory{timeout},
	}
	server.ServeHTTP(w, req)
}

func newRequestScope() *RequestScope {
	return &RequestScope{
		Serializer:          json.NewBasicNegotiatedSerializer(),
		StandardSerializers: nil,
		Resource:            schema.GroupVersionResource{},
		Kind:                schema.GroupVersionKind{},
		Subresource:         "",
		MetaGroupVersion:    schema.GroupVersion{},
		HubGroupVersion:     schema.GroupVersion{},
		MaxRequestBodyBytes: 0,
	}
}

// nothing will ever be sent down this channel
var neverExitWatch <-chan time.Time = make(chan time.Time)

// TimeoutFactory abstracts watch timeout logic for testing
type TimeoutFactory interface {
	TimeoutCh() (<-chan time.Time, func() bool)
}

// realTimeoutFactory implements timeoutFactory
type realTimeoutFactory struct {
	timeout time.Duration
}

// TimeoutCh returns a channel which will receive something when the watch times out,
// and a cleanup function to call when this happens.
func (w *realTimeoutFactory) TimeoutCh() (<-chan time.Time, func() bool) {
	if w.timeout == 0 {
		return neverExitWatch, func() bool { return false }
	}
	t := time.NewTimer(w.timeout)
	return t.C, t.Stop
}

type WatchServer struct {
	Watching        watch.Interface
	Scope           *RequestScope
	MediaType       string
	Framer          runtime.Framer
	Encoder         runtime.Encoder
	EmbeddedEncoder runtime.Encoder
	Fixup           func(runtime.Object) runtime.Object
	TimeoutFactory  TimeoutFactory
}

func (s *WatchServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if IsWebSocketRequest(req) {
		w.Header().Set("Content-Type", s.MediaType)
		websocket.Handler(s.handleWS).ServeHTTP(w, req)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		err := fmt.Errorf("unable to start watch - can't get http.Flusher: %#v", w)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	framer := s.Framer.NewFrameWriter(w)
	if framer == nil {
		err := fmt.Errorf("no stream framing support is available for media type %q", s.MediaType)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	var e streaming.Encoder
	var memoryAllocator runtime.MemoryAllocator
	if encoder, supportsAllocator := s.Encoder.(runtime.EncoderWithAllocator); supportsAllocator {
		memoryAllocator = runtime.AllocatorPool.Get().(*runtime.Allocator)
		defer runtime.AllocatorPool.Put(memoryAllocator)
		e = streaming.NewEncoderWithAllocator(framer, encoder, memoryAllocator)
	} else {
		e = streaming.NewEncoder(framer, s.Encoder)
	}

	timeoutCh, cleanup := s.TimeoutFactory.TimeoutCh()
	defer cleanup()

	w.Header().Set("Content-Type", s.MediaType)
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	var unknown meta.Unknown
	outEvent := &meta.WatchEvent{}
	buf := &bytes.Buffer{}
	ch := s.Watching.ResultChan()
	done := req.Context().Done()

	embeddedEncodeFn := s.EmbeddedEncoder.Encode
	if encoder, supportsAllocator := s.EmbeddedEncoder.(runtime.EncoderWithAllocator); supportsAllocator {
		if memoryAllocator == nil {
			memoryAllocator = runtime.AllocatorPool.Get().(*runtime.Allocator)
			defer runtime.AllocatorPool.Put(memoryAllocator)
		}
		embeddedEncodeFn = func(obj runtime.Object, w io.Writer) error {
			return encoder.EncodeWithAllocator(obj, w, memoryAllocator)
		}
	}

	for {
		select {
		case <-done:
			flog.Warn("watch handle done")
			return
		case <-timeoutCh:
			flog.Warn("watch handle timeout")
			return
		case event, ok := <-ch:
			if !ok {
				return
			}

			obj := s.Fixup(event.Object)
			if err := embeddedEncodeFn(obj, buf); err != nil {
				err = fmt.Errorf("unable to encode watch object %T: %v", obj, err)
				flog.Error(err)
				return
			}

			unknown.Raw = buf.Bytes()
			event.Object = &unknown

			outEvent = &meta.WatchEvent{}

			err := ConvertInternalEventToWatchEvent(&event, outEvent)
			if err != nil {
				err = fmt.Errorf("unable to convert watch object: %v", err)
				flog.Error(err)
				return
			}
			if err := e.Encode(outEvent); err != nil {
				err = fmt.Errorf("unable to encode watch object %T: %v (%#v)", outEvent, err, e)
				flog.Error(err)
				return
			}
			if len(ch) == 0 {
				flusher.Flush()
			}

			buf.Reset()
		}
	}
}

func (s *WatchServer) handleWS(ws *websocket.Conn) {
	defer ws.Close()
	done := make(chan struct{})

	go func() {
		defer parallelizer.HandleCrash()
		IgnoreReceives(ws, 0)
		close(done)
	}()

	var unknown meta.Unknown
	buf := &bytes.Buffer{}
	streamBuf := &bytes.Buffer{}
	ch := s.Watching.ResultChan()

	for {
		select {
		case <-done:
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			obj := s.Fixup(event.Object)
			if err := s.EmbeddedEncoder.Encode(obj, buf); err != nil {
				err = fmt.Errorf("unable to encode watch object %T: %v", obj, err)
				flog.Error(err)
				return
			}

			unknown.Raw = buf.Bytes()
			event.Object = &unknown

			outEvent := &meta.WatchEvent{}
			err := ConvertInternalEventToWatchEvent(&event, outEvent)
			if err != nil {
				flog.Error(err)
				return
			}
			if err := s.Encoder.Encode(outEvent, streamBuf); err != nil {
				err = fmt.Errorf("unable to encode event: %v", err)
				flog.Error(err)
				return
			}
			if err := websocket.Message.Send(ws, streamBuf.String()); err != nil {
				return
			}
			buf.Reset()
			streamBuf.Reset()
		}
	}
}

func ConvertInternalEventToWatchEvent(in *watch.Event, out *meta.WatchEvent) error {
	out.Type = string(in.Type)
	switch t := in.Object.(type) {
	case *meta.Unknown:
		out.Object.Raw = t.Raw
	case nil:
	default:
		out.Object.Object = in.Object
	}
	return nil
}

var (
	// connectionUpgradeRegex matches any Connection header value that includes upgrade
	connectionUpgradeRegex = regexp.MustCompile("(^|.*,\\s*)upgrade($|\\s*,)")
)

func IsWebSocketRequest(req *http.Request) bool {
	if !strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
		return false
	}

	return connectionUpgradeRegex.MatchString(strings.ToLower(req.Header.Get("Connection")))
}

func IgnoreReceives(ws *websocket.Conn, timeout time.Duration) {
	defer parallelizer.HandleCrash()
	var data []byte
	for {
		resetTimeout(ws, timeout)
		if err := websocket.Message.Receive(ws, &data); err != nil {
			return
		}
	}
}

func resetTimeout(ws *websocket.Conn, timeout time.Duration) {
	if timeout > 0 {
		_ = ws.SetDeadline(time.Now().Add(timeout))
	}
}

// RequestScope encapsulates common fields across all RESTFul handler methods.
type RequestScope struct {
	Serializer runtime.NegotiatedSerializer

	// StandardSerializers, if set, restricts which serializers can be used when
	// we aren't transforming the output (into Table or PartialObjectMetadata).
	// Used only by CRDs which do not yet support Protobuf.
	StandardSerializers []runtime.SerializerInfo

	Resource schema.GroupVersionResource
	Kind     schema.GroupVersionKind

	Subresource string

	MetaGroupVersion schema.GroupVersion

	// HubGroupVersion indicates what version objects read from etcd or incoming requests should be converted to for in-memory handling.
	HubGroupVersion schema.GroupVersion

	MaxRequestBodyBytes int64
}

func (scope *RequestScope) AllowsMediaTypeTransform(mimeType, mimeSubType string, gvk *schema.GroupVersionKind) bool {
	// some handlers like CRDs can't serve all the mime types that PartialObjectMetadata or Table can - if
	// gvk is nil (no conversion) allow StandardSerializers to further restrict the set of mime types.
	if gvk == nil {
		if len(scope.StandardSerializers) == 0 {
			return true
		}
		for _, info := range scope.StandardSerializers {
			if info.MediaTypeType == mimeType && info.MediaTypeSubType == mimeSubType {
				return true
			}
		}
		return false
	}

	return false
}

func (scope *RequestScope) AllowsServerVersion(version string) bool {
	return version == scope.MetaGroupVersion.Version
}

func (scope *RequestScope) AllowsStreamSchema(s string) bool {
	return s == "watch"
}
