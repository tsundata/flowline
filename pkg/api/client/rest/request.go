package rest

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	restclientwatch "github.com/tsundata/flowline/pkg/api/client/rest/watch"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"github.com/tsundata/flowline/pkg/runtime/serializer/streaming"
	"github.com/tsundata/flowline/pkg/util/flog"
	"github.com/tsundata/flowline/pkg/util/flowcontrol"
	"github.com/tsundata/flowline/pkg/util/net"
	"github.com/tsundata/flowline/pkg/watch"
	"golang.org/x/net/http2"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strings"
	"time"
)

var (
	// longThrottleLatency defines threshold for logging requests. All requests being
	// throttled (via the provided rateLimiter) for more than longThrottleLatency will
	// be logged.
	longThrottleLatency = 50 * time.Millisecond

	// extraLongThrottleLatency defines the threshold for logging requests at log level 2.
	extraLongThrottleLatency = 1 * time.Second
)

// HTTPClient is an interface for testing a request object.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// ResponseWrapper is an interface for getting a response.
// The response may be either accessed as a raw data (the whole output is put into memory) or as a stream.
type ResponseWrapper interface {
	DoRaw(context.Context) ([]byte, error)
	Stream(context.Context) (io.ReadCloser, error)
}

type requestRetryFunc func(maxRetries int) WithRetry

type Request struct {
	c *RESTClient

	rateLimiter flowcontrol.RateLimiter
	backoff     BackoffManager
	timeout     time.Duration
	maxRetries  int

	// generic components accessible via method setters
	verb       string
	pathPrefix string
	subpath    string
	params     url.Values
	headers    http.Header

	// structural elements of the request that are part of the Kubernetes API conventions
	resource     string
	resourceName string
	subresource  string

	// output
	err  error
	body io.Reader

	retryFn requestRetryFunc
}

var noBackoff = &NoBackoff{}

func defaultRequestRetryFn(maxRetries int) WithRetry {
	return &withRetry{maxRetries: maxRetries}
}

func NewRequest(c *RESTClient) *Request {
	var backoff BackoffManager
	if c.createBackoffMgr != nil {
		backoff = c.createBackoffMgr()
	}
	if backoff == nil {
		backoff = noBackoff
	}

	var pathPrefix string
	if c.base != nil {
		pathPrefix = path.Join("/", c.base.Path, c.versionedAPIPath)
	} else {
		pathPrefix = path.Join("/", c.versionedAPIPath)
	}

	var timeout time.Duration
	if c.Client != nil {
		timeout = c.Client.Timeout
	}

	r := &Request{
		c:           c,
		rateLimiter: c.rateLimiter,
		backoff:     backoff,
		timeout:     timeout,
		pathPrefix:  pathPrefix,
		maxRetries:  10,
		retryFn:     defaultRequestRetryFn,
	}

	switch {
	case len(c.content.AcceptContentTypes) > 0:
		r.SetHeader("Accept", c.content.AcceptContentTypes)
	case len(c.content.ContentType) > 0:
		r.SetHeader("Accept", c.content.ContentType+", */*")
	}
	return r
}

// NewRequestWithClient creates a Request with an embedded RESTClient for use in test scenarios.
func NewRequestWithClient(base *url.URL, versionedAPIPath string, content ClientContentConfig, client *http.Client) *Request {
	return NewRequest(&RESTClient{
		base:             base,
		versionedAPIPath: versionedAPIPath,
		content:          content,
		Client:           client,
	})
}

// Verb sets the verb this request will use.
func (r *Request) Verb(verb string) *Request {
	r.verb = verb
	return r
}

// Prefix adds segments to the relative beginning to the request path. These
// items will be placed before the optional Namespace, Resource, or Name sections.
// Setting AbsPath will clear any previously set Prefix segments
func (r *Request) Prefix(segments ...string) *Request {
	if r.err != nil {
		return r
	}
	r.pathPrefix = path.Join(r.pathPrefix, path.Join(segments...))
	return r
}

// Suffix appends segments to the end of the path. These items will be placed after the prefix and optional
// Namespace, Resource, or Name sections.
func (r *Request) Suffix(segments ...string) *Request {
	if r.err != nil {
		return r
	}
	r.subpath = path.Join(r.subpath, path.Join(segments...))
	return r
}

// Resource sets the resource to access (<resource>/[ns/<namespace>/]<name>)
func (r *Request) Resource(resource string) *Request {
	if r.err != nil {
		return r
	}
	if len(r.resource) != 0 {
		r.err = fmt.Errorf("resource already set to %q, cannot change to %q", r.resource, resource)
		return r
	}
	r.resource = resource
	return r
}

// BackOff sets the request's backoff manager to the one specified,
// or defaults to the stub implementation if nil is provided
func (r *Request) BackOff(manager BackoffManager) *Request {
	if manager == nil {
		r.backoff = &NoBackoff{}
		return r
	}

	r.backoff = manager
	return r
}

// Throttle receives a rate-limiter and sets or replaces an existing request limiter
func (r *Request) Throttle(limiter flowcontrol.RateLimiter) *Request {
	r.rateLimiter = limiter
	return r
}

// SubResource sets a sub-resource path which can be multiple segments after the resource
// name but before the suffix.
func (r *Request) SubResource(subresources ...string) *Request {
	if r.err != nil {
		return r
	}
	subresource := path.Join(subresources...)
	if len(r.subresource) != 0 {
		r.err = fmt.Errorf("subresource already set to %q, cannot change to %q", r.subresource, subresource)
		return r
	}
	r.subresource = subresource
	return r
}

// Name sets the name of a resource to access (<resource>/[ns/<namespace>/]<name>)
func (r *Request) Name(resourceName string) *Request {
	if r.err != nil {
		return r
	}
	if len(resourceName) == 0 {
		r.err = fmt.Errorf("resource name may not be empty")
		return r
	}
	if len(r.resourceName) != 0 {
		r.err = fmt.Errorf("resource name already set to %q, cannot change to %q", r.resourceName, resourceName)
		return r
	}
	r.resourceName = resourceName
	return r
}

// AbsPath overwrites an existing path with the segments provided. Trailing slashes are preserved
// when a single segment is passed.
func (r *Request) AbsPath(segments ...string) *Request {
	if r.err != nil {
		return r
	}
	r.pathPrefix = path.Join(r.c.base.Path, path.Join(segments...))
	if len(segments) == 1 && (len(r.c.base.Path) > 1 || len(segments[0]) > 1) && strings.HasSuffix(segments[0], "/") {
		// preserve any trailing slashes for legacy behavior
		r.pathPrefix += "/"
	}
	return r
}

// RequestURI overwrites existing path and parameters with the value of the provided server relative
// URI.
func (r *Request) RequestURI(uri string) *Request {
	if r.err != nil {
		return r
	}
	locator, err := url.Parse(uri)
	if err != nil {
		r.err = err
		return r
	}
	r.pathPrefix = locator.Path
	if len(locator.Query()) > 0 {
		if r.params == nil {
			r.params = make(url.Values)
		}
		for k, v := range locator.Query() {
			r.params[k] = v
		}
	}
	return r
}

// Param creates a query parameter with the given string value.
func (r *Request) Param(paramName, s string) *Request {
	if r.err != nil {
		return r
	}
	return r.setParam(paramName, s)
}

func (r *Request) setParam(paramName, value string) *Request {
	if r.params == nil {
		r.params = make(url.Values)
	}
	r.params[paramName] = append(r.params[paramName], value)
	return r
}

func (r *Request) SetHeader(key string, values ...string) *Request {
	if r.headers == nil {
		r.headers = http.Header{}
	}
	r.headers.Del(key)
	for _, value := range values {
		r.headers.Add(key, value)
	}
	return r
}

// Timeout makes the request use the given duration as an overall timeout for the
// request. Additionally, if set passes the value as "timeout" parameter in URL.
func (r *Request) Timeout(d time.Duration) *Request {
	if r.err != nil {
		return r
	}
	r.timeout = d
	return r
}

// MaxRetries makes the request use the given integer as a ceiling of retrying upon receiving
// "Retry-After" headers and 429 status-code in the response. The default is 10 unless this
// function is specifically called with a different value.
// A zero maxRetries prevent it from doing retires and return an error immediately.
func (r *Request) MaxRetries(maxRetries int) *Request {
	if maxRetries < 0 {
		maxRetries = 0
	}
	r.maxRetries = maxRetries
	return r
}

// Body makes the request use obj as the body. Optional.
// If obj is a string, try to read a file of that name.
// If obj is a []byte, send it directly.
// If obj is an io.Reader, use it directly.
// If obj is a runtime.Object, marshal it correctly, and set Content-Type header.
// If obj is a runtime.Object and nil, do nothing.
// Otherwise, set an error.
func (r *Request) Body(obj interface{}) *Request {
	if r.err != nil {
		return r
	}
	switch t := obj.(type) {
	case string:
		data, err := ioutil.ReadFile(t)
		if err != nil {
			r.err = err
			return r
		}
		flogBody("Request Body", data)
		r.body = bytes.NewReader(data)
	case []byte:
		flogBody("Request Body", t)
		r.body = bytes.NewReader(t)
	case io.Reader:
		r.body = t
	case runtime.Object:
		// callers may pass typed interface pointers, therefore we must check nil with reflection
		if reflect.ValueOf(t).IsNil() {
			return r
		}
		encoder, err := r.c.content.Negotiator.Encoder(r.c.content.ContentType, nil)
		if err != nil {
			r.err = err
			return r
		}
		data, err := runtime.Encode(encoder, t)
		if err != nil {
			r.err = err
			return r
		}
		flogBody("Request Body", data)
		r.body = bytes.NewReader(data)
		r.SetHeader("Content-Type", r.c.content.ContentType)
	default:
		r.err = fmt.Errorf("unknown type used for body: %+v", obj)
	}
	return r
}

// flogBody logs a body output that could be either JSON or protobuf. It explicitly guards against
// allocating a new string for the body output unless necessary. Uses a simple heuristic to determine
// whether the body is printable.
func flogBody(prefix string, body []byte) {
	if bytes.IndexFunc(body, func(r rune) bool {
		return r < 0x0a
	}) != -1 {
		flog.Infof("%s:\n%s", prefix, hex.Dump(body))
	} else {
		flog.Infof("%s: %s", prefix, string(body))
	}
}

// URL returns the current working URL.
func (r *Request) URL() *url.URL {
	p := r.pathPrefix
	if len(r.resource) != 0 {
		p = path.Join(p, strings.ToLower(r.resource))
	}
	// Join trims trailing slashes, so preserve r.pathPrefix's trailing slash for backwards compatibility if nothing was changed
	if len(r.resourceName) != 0 || len(r.subpath) != 0 || len(r.subresource) != 0 {
		p = path.Join(p, r.resourceName, r.subresource, r.subpath)
	}

	finalURL := &url.URL{}
	if r.c.base != nil {
		*finalURL = *r.c.base
	}
	finalURL.Path = p

	query := url.Values{}
	for key, values := range r.params {
		for _, value := range values {
			query.Add(key, value)
		}
	}

	// timeout is handled specially here.
	if r.timeout != 0 {
		query.Set("timeout", r.timeout.String())
	}
	finalURL.RawQuery = query.Encode()
	return finalURL
}

func (r *Request) tryThrottleWithInfo(ctx context.Context, retryInfo string) error {
	if r.rateLimiter == nil {
		return nil
	}

	now := time.Now()

	err := r.rateLimiter.Wait(ctx)
	if err != nil {
		err = fmt.Errorf("client rate limiter Wait returned an error: %w", err)
	}
	latency := time.Since(now)

	var message string
	switch {
	case len(retryInfo) > 0:
		message = fmt.Sprintf("Waited for %v, %s - request: %s:%s", latency, retryInfo, r.verb, r.URL().String())
	default:
		message = fmt.Sprintf("Waited for %v due to client-side throttling, not priority and fairness, request: %s:%s", latency, r.verb, r.URL().String())
	}

	if latency > longThrottleLatency {
		flog.Info(message)
	}

	return err
}

func (r *Request) tryThrottle(ctx context.Context) error {
	return r.tryThrottleWithInfo(ctx, "")
}

// Watch attempts to begin watching the requested location.
// Returns a watch.Interface, or an error.
func (r *Request) Watch(ctx context.Context) (watch.Interface, error) {
	// We specifically don't want to rate limit watches, so we
	// don't use r.rateLimiter here.
	if r.err != nil {
		return nil, r.err
	}

	client := r.c.Client
	if client == nil {
		client = http.DefaultClient
	}

	isErrRetryableFunc := func(request *http.Request, err error) bool {
		// The watch stream mechanism handles many common partial data errors, so closed
		// connections can be retried in many cases.
		if net.IsProbableEOF(err) || net.IsTimeout(err) {
			return true
		}
		return false
	}
	retry := r.retryFn(r.maxRetries)
	u := r.URL().String()
	for {
		if err := retry.Before(ctx, r); err != nil {
			return nil, retry.WrapPreviousError(err)
		}

		req, err := r.newHTTPRequest(ctx)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		retry.After(ctx, r, resp, err)
		if err == nil && resp.StatusCode == http.StatusOK {
			return r.newStreamWatcher(resp)
		}

		done, transformErr := func() (bool, error) {
			defer readAndCloseResponseBody(resp)

			if retry.IsNextRetry(ctx, r, req, resp, err, isErrRetryableFunc) {
				return false, nil
			}

			if resp == nil {
				// the server must have sent us an error in 'err'
				return true, nil
			}
			if result := r.transformResponse(resp, req); result.err != nil {
				return true, result.err
			}
			return true, fmt.Errorf("for request %s, got status: %v", u, resp.StatusCode)
		}()
		if done {
			if isErrRetryableFunc(req, err) {
				return watch.NewEmptyWatch(), nil
			}
			if err == nil {
				// if the server sent us an HTTP Response object,
				// we need to return the error object from that.
				err = transformErr
			}
			return nil, retry.WrapPreviousError(err)
		}
	}
}

// transformResponse converts an API response into a structured API object
func (r *Request) transformResponse(resp *http.Response, req *http.Request) Result {
	var body []byte
	if resp.Body != nil {
		data, err := ioutil.ReadAll(resp.Body)
		switch err.(type) {
		case nil:
			body = data
		case http2.StreamError:
			// This is trying to catch the scenario that the server may close the connection when sending the
			// response body. This can be caused by server timeout due to a slow network connection.
			flog.Infof("Stream error %#v when reading response body, may be caused by closed connection.", err)
			streamErr := fmt.Errorf("stream error when reading response body, may be caused by closed connection. Please retry. Original error: %w", err)
			return Result{
				err: streamErr,
			}
		default:
			flog.Errorf("Unexpected error when reading response body: %v", err)
			unexpectedErr := fmt.Errorf("unexpected error when reading response body. Please retry. Original error: %w", err)
			return Result{
				err: unexpectedErr,
			}
		}
	}

	flogBody("Response Body", body)

	// verify the content type is accurate
	var decoder runtime.Decoder
	contentType := resp.Header.Get("Content-Type")
	if len(contentType) == 0 {
		contentType = r.c.content.ContentType
	}
	if len(contentType) > 0 {
		var err error
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			return Result{err: err}
		}
		decoder, err = r.c.content.Negotiator.Decoder(mediaType, params)
		if err != nil {
			// if we fail to negotiate a decoder, treat this as an unstructured error
			switch {
			case resp.StatusCode == http.StatusSwitchingProtocols:
				// no-op, we've been upgraded
			case resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusPartialContent:
				return Result{err: r.transformUnstructuredResponseError(resp, req, body)}
			}
			return Result{
				body:        body,
				contentType: contentType,
				statusCode:  resp.StatusCode,
			}
		}
	}

	switch {
	case resp.StatusCode == http.StatusSwitchingProtocols:
		// no-op, we've been upgraded
	case resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusPartialContent:
		// calculate an unstructured error from the response which the Result object may use if the caller
		// did not return a structured error.
		retryAfter, _ := retryAfterSeconds(resp)
		err := r.newUnstructuredResponseError(body, isTextResponse(resp), resp.StatusCode, req.Method, retryAfter)
		return Result{
			body:        body,
			contentType: contentType,
			statusCode:  resp.StatusCode,
			decoder:     decoder,
			err:         err,
		}
	}

	return Result{
		body:        body,
		contentType: contentType,
		statusCode:  resp.StatusCode,
		decoder:     decoder,
	}
}

// transformUnstructuredResponseError handles an error from the server that is not in a structured form.
// It is expected to transform any response that is not recognizable as a clear server sent error from the
// K8S API using the information provided with the request. In practice, HTTP proxies and client libraries
// introduce a level of uncertainty to the responses returned by servers that in common use result in
// unexpected responses. The rough structure is:
//
// 1. Assume the server sends you something sane - JSON + well defined error objects + proper codes
//    - this is the happy path
//    - when you get this output, trust what the server sends
// 2. Guard against empty fields / bodies in received JSON and attempt to cull sufficient info from them to
//    generate a reasonable facsimile of the original failure.
//    - Be sure to use a distinct error type or flag that allows a client to distinguish between this and error 1 above
// 3. Handle true disconnect failures / completely malformed data by moving up to a more generic client error
// 4. Distinguish between various connection failures like SSL certificates, timeouts, proxy errors, unexpected
//    initial contact, the presence of mismatched body contents from posted content types
//    - Give these a separate distinct error type and capture as much as possible of the original message
func (r *Request) transformUnstructuredResponseError(resp *http.Response, req *http.Request, body []byte) error {
	if body == nil && resp.Body != nil {
		if data, err := ioutil.ReadAll(&io.LimitedReader{R: resp.Body, N: maxUnstructuredResponseTextBytes}); err == nil {
			body = data
		}
	}
	retryAfter, _ := retryAfterSeconds(resp)
	return r.newUnstructuredResponseError(body, isTextResponse(resp), resp.StatusCode, req.Method, retryAfter)
}

// isTextResponse returns true if the response appears to be a textual media type.
func isTextResponse(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	if len(contentType) == 0 {
		return true
	}
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return strings.HasPrefix(media, "text/")
}

// maxUnstructuredResponseTextBytes is an upper bound on how much output to include in the unstructured error.
const maxUnstructuredResponseTextBytes = 2048

// newUnstructuredResponseError instantiates the appropriate generic error for the provided input. It also logs the body.
func (r *Request) newUnstructuredResponseError(body []byte, isTextResponse bool, statusCode int, method string, retryAfter int) error {
	// cap the amount of output we create
	if len(body) > maxUnstructuredResponseTextBytes {
		body = body[:maxUnstructuredResponseTextBytes]
	}

	message := "unknown"
	if isTextResponse {
		message = strings.TrimSpace(string(body))
	}
	var groupResource schema.GroupResource
	if len(r.resource) > 0 {
		groupResource.Group = r.c.content.GroupVersion.Group
		groupResource.Resource = r.resource
	}
	return fmt.Errorf("response error %d %s %v %s %s %d",
		statusCode,
		method,
		groupResource,
		r.resourceName,
		message,
		retryAfter,
	)
}

func (r *Request) newStreamWatcher(resp *http.Response) (watch.Interface, error) {
	contentType := resp.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		flog.Infof("Unexpected content type from the server: %q: %v", contentType, err)
	}
	objectDecoder, streamingSerializer, framer, err := r.c.content.Negotiator.StreamDecoder(mediaType, params)
	if err != nil {
		return nil, err
	}

	frameReader := framer.NewFrameReader(resp.Body)
	watchEventDecoder := streaming.NewDecoder(frameReader, streamingSerializer)

	return watch.NewStreamWatcher(
		restclientwatch.NewDecoder(watchEventDecoder, objectDecoder),
		// use 500 to indicate that the cause of the error is unknown - other error codes
		// are more specific to HTTP interactions, and set a reason
		NewClientErrorReporter(http.StatusInternalServerError, r.verb, "ClientWatchDecoding"),
	), nil
}

// Stream formats and executes the request, and offers streaming of the response.
// Returns io.ReadCloser which could be used for streaming of the response, or an error
// Any non-2xx http status code causes an error.  If we get a non-2xx code, we try to convert the body into an APIStatus object.
// If we can, we return that as an error.  Otherwise, we create an error that lists the http status and the content of the response.
func (r *Request) Stream(ctx context.Context) (io.ReadCloser, error) {
	if r.err != nil {
		return nil, r.err
	}

	if err := r.tryThrottle(ctx); err != nil {
		return nil, err
	}

	client := r.c.Client
	if client == nil {
		client = http.DefaultClient
	}

	retry := r.retryFn(r.maxRetries)
	u := r.URL().String()
	for {
		if err := retry.Before(ctx, r); err != nil {
			return nil, err
		}

		req, err := r.newHTTPRequest(ctx)
		if err != nil {
			return nil, err
		}
		if r.body != nil {
			req.Body = ioutil.NopCloser(r.body)
		}
		resp, err := client.Do(req)
		retry.After(ctx, r, resp, err)
		if err != nil {
			// we only retry on an HTTP response with 'Retry-After' header
			return nil, err
		}

		switch {
		case (resp.StatusCode >= 200) && (resp.StatusCode < 300):
			return resp.Body, nil

		default:
			done, transformErr := func() (bool, error) {
				defer resp.Body.Close()

				if retry.IsNextRetry(ctx, r, req, resp, err, neverRetryError) {
					return false, nil
				}
				result := r.transformResponse(resp, req)
				if err := result.Error(); err != nil {
					return true, err
				}
				return true, fmt.Errorf("%d while accessing %v: %s", result.statusCode, u, string(result.body))
			}()
			if done {
				return nil, transformErr
			}
		}
	}
}

// requestPreflightCheck looks for common programmer errors on Request.
//
// We tackle here two programmer mistakes. The first one is to try to create
// something(POST) using an empty string as namespace with namespaceSet as
// true. If namespaceSet is true then namespace should also be defined. The
// second mistake is, when under the same circumstances, the programmer tries
// to GET, PUT or DELETE a named resource(resourceName != ""), again, if
// namespaceSet is true then namespace must not be empty.
func (r *Request) requestPreflightCheck() error {
	switch r.verb {
	case "POST":
		return fmt.Errorf("an empty namespace may not be set during creation")
	case "GET", "PUT", "DELETE":
		if len(r.resourceName) > 0 {
			return fmt.Errorf("an empty namespace may not be set when a resource name is provided")
		}
	}
	return nil
}

func (r *Request) newHTTPRequest(ctx context.Context) (*http.Request, error) {
	u := r.URL().String()
	req, err := http.NewRequest(r.verb, u, r.body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header = r.headers
	return req, nil
}

// request connects to the server and invokes the provided function when a server response is
// received. It handles retry behavior and up front validation of requests. It will invoke
// fn at most once. It will return an error if a problem occurred prior to connecting to the
// server - the provided function is responsible for handling server errors.
func (r *Request) request(ctx context.Context, fn func(*http.Request, *http.Response)) error {
	if r.err != nil {
		flog.Infof("Error in request: %v", r.err)
		return r.err
	}

	if err := r.requestPreflightCheck(); err != nil {
		return err
	}

	client := r.c.Client
	if client == nil {
		client = http.DefaultClient
	}

	// Throttle the first try before setting up the timeout configured on the
	// client. We don't want a throttled client to return timeouts to callers
	// before it makes a single request.
	if err := r.tryThrottle(ctx); err != nil {
		return err
	}

	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	isErrRetryableFunc := func(req *http.Request, err error) bool {
		// "Connection reset by peer" or "apiserver is shutting down" are usually a transient errors.
		// Thus in case of "GET" operations, we simply retry it.
		// We are not automatically retrying "write" operations, as they are not idempotent.
		if req.Method != "GET" {
			return false
		}
		// For connection errors and apiserver shutdown errors retry.
		if net.IsConnectionReset(err) || net.IsProbableEOF(err) {
			return true
		}
		return false
	}

	// Right now we make about ten retry attempts if we get a Retry-After response.
	retry := r.retryFn(r.maxRetries)
	for {
		if err := retry.Before(ctx, r); err != nil {
			return retry.WrapPreviousError(err)
		}
		req, err := r.newHTTPRequest(ctx)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		// The value -1 or a value of 0 with a non-nil Body indicates that the length is unknown.
		// https://pkg.go.dev/net/http#Request
		retry.After(ctx, r, resp, err)

		done := func() bool {
			defer readAndCloseResponseBody(resp)

			// if the the server returns an error in err, the response will be nil.
			f := func(req *http.Request, resp *http.Response) {
				if resp == nil {
					return
				}
				fn(req, resp)
			}

			if retry.IsNextRetry(ctx, r, req, resp, err, isErrRetryableFunc) {
				return false
			}

			f(req, resp)
			return true
		}()
		if done {
			return retry.WrapPreviousError(err)
		}
	}
}

// Do formats and executes the request. Returns a Result object for easy response
// processing.
//
// Error type:
//  * If the server responds with a status: *errors.StatusError or *errors.UnexpectedObjectError
//  * http.Client.Do errors are returned directly.
func (r *Request) Do(ctx context.Context) Result {
	var result Result
	err := r.request(ctx, func(req *http.Request, resp *http.Response) {
		result = r.transformResponse(resp, req)
	})
	if err != nil {
		return Result{err: err}
	}
	return result
}

// DoRaw executes the request but does not process the response body.
func (r *Request) DoRaw(ctx context.Context) ([]byte, error) {
	var result Result
	err := r.request(ctx, func(req *http.Request, resp *http.Response) {
		result.body, result.err = ioutil.ReadAll(resp.Body)
		flogBody("Response Body", result.body)
		if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusPartialContent {
			result.err = r.transformUnstructuredResponseError(resp, req, result.body)
		}
	})
	if err != nil {
		return nil, err
	}
	return result.body, result.err
}

// VersionedParams will take the provided object, serialize it to a map[string][]string using the
// implicit RESTClient API version and the default parameter codec, and then add those as parameters
// to the request. Use this to provide versioned query parameters from client libraries.
// VersionedParams will not write query parameters that have omitempty set and are empty. If a
// parameter has already been set it is appended to (Params and VersionedParams are additive).
func (r *Request) VersionedParams(obj runtime.Object, codec runtime.ParameterCodec) *Request {
	return r.SpecificallyVersionedParams(obj, codec, r.c.content.GroupVersion)
}

func (r *Request) SpecificallyVersionedParams(obj runtime.Object, codec runtime.ParameterCodec, version schema.GroupVersion) *Request {
	if r.err != nil {
		return r
	}
	params, err := codec.EncodeParameters(obj, version)
	if err != nil {
		r.err = err
		return r
	}
	for k, v := range params {
		if r.params == nil {
			r.params = make(url.Values)
		}
		r.params[k] = append(r.params[k], v...)
	}
	return r
}

// Result contains the result of calling Request.Do().
type Result struct {
	body        []byte
	contentType string
	err         error
	statusCode  int

	decoder runtime.Decoder
}

// Raw returns the raw result.
func (r Result) Raw() ([]byte, error) {
	return r.body, r.err
}

// Get returns the result as an object, which means it passes through the decoder.
// If the returned object is of type Status and has .Status != StatusSuccess, the
// additional information in Status will be used to enrich the error.
func (r Result) Get() (runtime.Object, error) {
	if r.err != nil {
		// Check whether the result has a Status object in the body and prefer that.
		return nil, r.Error()
	}
	if r.decoder == nil {
		return nil, fmt.Errorf("serializer for %s doesn't exist", r.contentType)
	}

	// decode, but if the result is Status return that as an error instead.
	out, _, err := r.decoder.Decode(r.body, nil, nil)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StatusCode returns the HTTP status code of the request. (Only valid if no
// error was returned.)
func (r Result) StatusCode(statusCode *int) Result {
	*statusCode = r.statusCode
	return r
}

// Into stores the result into obj, if possible. If obj is nil it is ignored.
// If the returned object is of type Status and has .Status != StatusSuccess, the
// additional information in Status will be used to enrich the error.
func (r Result) Into(obj runtime.Object) error {
	if r.err != nil {
		// Check whether the result has a Status object in the body and prefer that.
		return r.Error()
	}
	if r.decoder == nil {
		return fmt.Errorf("serializer for %s doesn't exist", r.contentType)
	}
	if len(r.body) == 0 {
		return fmt.Errorf("0-length response with status code: %d and content type: %s",
			r.statusCode, r.contentType)
	}

	out, _, err := r.decoder.Decode(r.body, nil, obj)
	if err != nil || out == obj {
		return err
	}
	return nil
}

// WasCreated updates the provided bool pointer to whether the server returned
// 201 created or a different response.
func (r Result) WasCreated(wasCreated *bool) Result {
	*wasCreated = r.statusCode == http.StatusCreated
	return r
}

// Error returns the error executing the request, nil if no error occurred.
// If the returned object is of type Status and has Status != StatusSuccess, the
// additional information in Status will be used to enrich the error.
// See the Request.Do() comment for what errors you might get.
func (r Result) Error() error {
	// if we have received an unexpected server error, and we have a body and decoder, we can try to extract
	// a Status object.
	if r.err == nil || len(r.body) == 0 || r.decoder == nil {
		return r.err
	}

	// attempt to convert the body into a Status object
	// to be backwards compatible with old servers that do not return a version, default to "v1"
	_, _, err := r.decoder.Decode(r.body, &schema.GroupVersionKind{Version: "v1"}, nil)
	if err != nil {
		flog.Infof("body was not decodable (unable to check for Status): %v", err)
		return r.err
	}
	return r.err
}
