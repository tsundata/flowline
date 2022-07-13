package client

import (
	"context"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/constant"
	"net/http"
	"time"
)

type Result struct {
	body       []byte
	err        error
	statusCode int
}

type RestClient struct {
	*resty.Client

	ctx      context.Context
	req      *resty.Request
	group    string
	version  string
	resource string
	uid      string
	obj      runtime.Object
	operate  string
}

func New(baseURL string) *RestClient {
	hc := &http.Client{
		Timeout: 60 * time.Second, //todo
	}
	cli := resty.NewWithClient(hc)
	cli.SetBaseURL(baseURL)
	restCli := &RestClient{Client: cli}
	restCli.group = constant.GroupName
	restCli.version = constant.Version
	return restCli
}

func (c *RestClient) Request(ctx context.Context) *RestClient {
	c.req = c.R()
	c.req.SetContext(ctx)
	c.uid = ""
	c.obj = nil
	return c
}

func (c *RestClient) resourcePrefix() string {
	return fmt.Sprintf("%s/%s/%s", c.group, c.version, c.resource)
}

func (c *RestClient) Connection() *RestClient {
	c.resource = "connection"
	return c
}

func (c *RestClient) Dag() *RestClient {
	c.resource = "dag"
	return c
}

func (c *RestClient) Event() *RestClient {
	c.resource = "event"
	return c
}

func (c *RestClient) Function() *RestClient {
	c.resource = "function"
	return c
}

func (c *RestClient) Job() *RestClient {
	c.resource = "job"
	return c
}

func (c *RestClient) Role() *RestClient {
	c.resource = "role"
	return c
}

func (c *RestClient) RoleBinding() *RestClient {
	c.resource = "rolebinding"
	return c
}

func (c *RestClient) User() *RestClient {
	c.resource = "user"
	return c
}

func (c *RestClient) Variable() *RestClient {
	c.resource = "variable"
	return c
}

func (c *RestClient) Worker() *RestClient {
	c.resource = "worker"
	return c
}

func (c *RestClient) Workflow() *RestClient {
	c.resource = "workflow"
	return c
}

func (c *RestClient) List() *RestClient {
	c.operate = "list"
	return c
}

func (c *RestClient) UID(uid string) *RestClient {
	c.uid = uid
	return c
}

func (c *RestClient) Object(obj runtime.Object) *RestClient {
	c.obj = obj
	return c
}

func (c *RestClient) Get() *RestClient {
	c.req.Method = http.MethodGet
	return c
}

func (c *RestClient) Create() *RestClient {
	c.req.Method = http.MethodPost
	return c
}

func (c *RestClient) Update() *RestClient {
	c.req.Method = http.MethodPut
	return c
}

func (c *RestClient) Delete() *RestClient {
	c.req.Method = http.MethodDelete
	return c
}

func (c *RestClient) Result() *Result {
	path := c.resourcePrefix()
	if len(c.uid) > 0 {
		path = path + "/" + c.uid
	}
	if len(c.operate) > 0 {
		path = path + "/" + c.operate
	}
	res := &Result{}
	switch c.req.Method {
	case http.MethodGet:
		resp, err := c.req.Get(path)
		res.body = resp.Body()
		res.statusCode = resp.StatusCode()
		res.err = err
	case http.MethodPost:
		resp, err := c.req.SetBody(c.obj).Post(path)
		res.body = resp.Body()
		res.statusCode = resp.StatusCode()
		res.err = err
	case http.MethodPut:
		resp, err := c.req.SetBody(c.obj).Delete(path)
		res.body = resp.Body()
		res.statusCode = resp.StatusCode()
		res.err = err
	case http.MethodDelete:
		resp, err := c.req.Delete(path)
		res.body = resp.Body()
		res.statusCode = resp.StatusCode()
		res.err = err
	default:
		return &Result{}
	}
	return res
}

func (r *Result) Into(obj runtime.Object) error {
	if r.err != nil {
		return r.err
	}
	jsonCoder := runtime.JsonCoder{}
	_, _, err := jsonCoder.Decode(r.body, nil, obj)
	return err
}