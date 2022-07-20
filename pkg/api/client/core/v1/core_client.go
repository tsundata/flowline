package v1

import (
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"github.com/tsundata/flowline/pkg/runtime/serializer/json"
	"net/http"
)

type CoreV1Interface interface {
	RESTClient() rest.Interface
	WorkerGetter
	StageGetter
}

type CoreV1Client struct {
	restClient rest.Interface
}

func (c *CoreV1Client) Worker() WorkerInterface {
	return newWorker(c)
}

func (c *CoreV1Client) Stage() StageInterface {
	return newStage(c)
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *CoreV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}

func NewForConfig(c *rest.Config, h *http.Client) (*CoreV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientForConfigAndClient(&config, h)
	if err != nil {
		return nil, err
	}
	return &CoreV1Client{client}, nil
}

func New(c rest.Interface) *CoreV1Client {
	return &CoreV1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	jsonCoder := runtime.JsonCoder{}
	gv := schema.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/api"
	config.NegotiatedSerializer = runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{
		StreamSerializer: &runtime.StreamSerializerInfo{
			EncodesAsText: true,
			Framer:        json.Framer,
			Serializer:    runtime.NewBase64Serializer(jsonCoder, jsonCoder),
		},
		EncodesAsText: true,
	})

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultUserAgent()
	}

	return nil
}

// NewForConfigAndClient creates a new CoreV1Client for the given config and http client.
// Note the http client provided takes precedence over the configured transport values.
func NewForConfigAndClient(c *rest.Config, h *http.Client) (*CoreV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientForConfigAndClient(&config, h)
	if err != nil {
		return nil, err
	}
	return &CoreV1Client{client}, nil
}
