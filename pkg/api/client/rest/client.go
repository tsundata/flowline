package rest

import (
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"github.com/tsundata/flowline/pkg/util/flowcontrol"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
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

// NewRESTClient creates a new RESTClient. This client performs generic REST functions
// such as Get, Put, Post, and Delete on specified paths.
func NewRESTClient(baseURL *url.URL, versionedAPIPath string, config ClientContentConfig, rateLimiter flowcontrol.RateLimiter, client *http.Client) (*RESTClient, error) {
	if len(config.ContentType) == 0 {
		config.ContentType = "application/json"
	}

	base := *baseURL
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	base.RawQuery = ""
	base.Fragment = ""

	return &RESTClient{
		base:             &base,
		versionedAPIPath: versionedAPIPath,
		content:          config,
		createBackoffMgr: readExpBackoffConfig,
		rateLimiter:      rateLimiter,

		Client: client,
	}, nil
}

const (
	// Environment variables: Note that the duration should be long enough that the backoff
	// persists for some reasonable time (i.e. 120 seconds).  The typical base might be "1".
	envBackoffBase     = "CLIENT_BACKOFF_BASE"
	envBackoffDuration = "CLIENT_BACKOFF_DURATION"
)

// readExpBackoffConfig handles the internal logic of determining what the
// backoff policy is.  By default if no information is available, NoBackoff.
func readExpBackoffConfig() BackoffManager {
	backoffBase := os.Getenv(envBackoffBase)
	backoffDuration := os.Getenv(envBackoffDuration)

	backoffBaseInt, errBase := strconv.ParseInt(backoffBase, 10, 64)
	backoffDurationInt, errDuration := strconv.ParseInt(backoffDuration, 10, 64)
	if errBase != nil || errDuration != nil {
		return &NoBackoff{}
	}
	return &URLBackoff{
		Backoff: flowcontrol.NewBackOff(
			time.Duration(backoffBaseInt)*time.Second,
			time.Duration(backoffDurationInt)*time.Second)}
}
