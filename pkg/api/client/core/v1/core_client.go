package v1

import (
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"github.com/tsundata/flowline/pkg/runtime/serializer/json"
	"net/http"
)

type CoreV1Interface interface {
	RESTClient() rest.Interface
	WorkflowGetter
	JobGetter
	WorkerGetter
	StageGetter
	CodeGetter
	VariableGetter
	ConnectionGetter
}

type CoreV1Client struct {
	restClient rest.Interface
}

func (c *CoreV1Client) Workflow() WorkflowInterface {
	return newWorkflow(c)
}

func (c *CoreV1Client) Job() JobInterface {
	return newJob(c)
}

func (c *CoreV1Client) Worker() WorkerInterface {
	return newWorker(c)
}

func (c *CoreV1Client) Stage() StageInterface {
	return newStage(c)
}

func (c *CoreV1Client) Code() CodeInterface {
	return newCode(c)
}

func (c *CoreV1Client) Variable() VariableInterface {
	return newVariable(c)
}

func (c *CoreV1Client) Connection() ConnectionInterface {
	return newConnection(c)
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
	gv := schema.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/api"
	config.NegotiatedSerializer = json.NewBasicNegotiatedSerializer()

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
