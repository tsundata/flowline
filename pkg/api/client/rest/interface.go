package rest

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"github.com/tsundata/flowline/pkg/util/flowcontrol"
	"net/http"
	"net/url"
)

type Interface interface {
	GetRateLimiter() flowcontrol.RateLimiter
	Verb(verb string) *Request
	Post() *Request
	Put() *Request
	Patch(pt string) *Request
	Get() *Request
	Delete() *Request
	APIVersion() schema.GroupVersion
}

type RESTClient struct {
	// base is the root URL for all invocations of the client
	base *url.URL
	// versionedAPIPath is a path segment connecting the base URL to the resource root
	versionedAPIPath string

	// content describes how a RESTClient encodes and decodes responses.
	content ClientContentConfig

	// creates BackoffManager that is passed to requests.
	createBackoffMgr func() BackoffManager

	// rateLimiter is shared among all requests created by this client unless specifically
	// overridden.
	rateLimiter flowcontrol.RateLimiter

	// Set specific behavior of the client.  If not set http.DefaultClient will be used.
	Client *http.Client
}

func (c *RESTClient) GetRateLimiter() flowcontrol.RateLimiter {
	if c == nil {
		return nil
	}
	return c.rateLimiter
}

func (c *RESTClient) Verb(verb string) *Request {
	return NewRequest(c).Verb(verb)
}

func (c *RESTClient) Post() *Request {
	return c.Verb(http.MethodPost)
}

func (c *RESTClient) Put() *Request {
	return c.Verb(http.MethodPut)
}

func (c *RESTClient) Patch(pt string) *Request {
	return c.Verb(http.MethodPatch).SetHeader("Content-Type", pt)
}

func (c *RESTClient) Get() *Request {
	return c.Verb(http.MethodPost)
}

func (c *RESTClient) Delete() *Request {
	return c.Verb(http.MethodDelete)
}

func (c *RESTClient) APIVersion() schema.GroupVersion {
	return c.content.GroupVersion
}

type ClientContentConfig struct {
	// AcceptContentTypes specifies the types the client will accept and is optional.
	// If not set, ContentType will be used to define the Accept header
	AcceptContentTypes string
	// ContentType specifies the wire format used to communicate with the server.
	// This value will be set as the Accept header on requests made to the server if
	// AcceptContentTypes is not set, and as the default content type on any object
	// sent to the server. If not set, "application/json" is used.
	ContentType string
	// GroupVersion is the API version to talk to. Must be provided when initializing
	// a RESTClient directly. When initializing a Client, will be set with the default
	// code version. This is used as the default group version for VersionedParams.
	GroupVersion schema.GroupVersion
	// Negotiator is used for obtaining encoders and decoders for multiple
	// supported media types.
	Negotiator runtime.ClientNegotiator
}
