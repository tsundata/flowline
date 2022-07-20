package v1

import "github.com/tsundata/flowline/pkg/api/client/rest"

type CoreV1Interface interface {
	RESTClient() rest.Interface
	StagesGetter
}

type CoreV1Client struct {
	restClient rest.Interface
}

func (c *CoreV1Client) Stages() StageInterface {
	return newStages(c)
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *CoreV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
