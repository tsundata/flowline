package v1

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/watch"
	"time"
)

type ConnectionGetter interface {
	Connection() ConnectionInterface
}

type ConnectionInterface interface {
	Create(ctx context.Context, connection *meta.Connection, opts meta.CreateOptions) (*meta.Connection, error)
	Update(ctx context.Context, connection *meta.Connection, opts meta.UpdateOptions) (*meta.Connection, error)
	Delete(ctx context.Context, name string, opts meta.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error
	Get(ctx context.Context, name string, opts meta.GetOptions) (*meta.Connection, error)
	List(ctx context.Context, opts meta.ListOptions) (*meta.ConnectionList, error)
	Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt string, data []byte, opts meta.PatchOptions, subresources ...string) (result *meta.Connection, err error)
}

type connection struct {
	client rest.Interface
}

func newConnection(c *CoreV1Client) *connection {
	return &connection{client: c.RESTClient()}
}

func (c *connection) Create(ctx context.Context, connection *meta.Connection, _ meta.CreateOptions) (*meta.Connection, error) {
	var result = &meta.Connection{}
	var err = c.client.Post().
		Resource("connection").
		Body(connection).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *connection) Update(ctx context.Context, connection *meta.Connection, _ meta.UpdateOptions) (*meta.Connection, error) {
	var result = &meta.Connection{}
	var err = c.client.Put().
		Resource("connection").
		Name(connection.UID).
		Body(connection).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *connection) Delete(ctx context.Context, name string, opts meta.DeleteOptions) error {
	return c.client.Delete().
		Resource("connection").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *connection) DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}

	return c.client.Delete().
		Resource("connection").
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *connection) Get(ctx context.Context, name string, _ meta.GetOptions) (*meta.Connection, error) {
	var result = &meta.Connection{}
	err := c.client.Get().
		Resource("connection").
		Name(name).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *connection) List(ctx context.Context, opts meta.ListOptions) (*meta.ConnectionList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}

	var result = &meta.ConnectionList{}
	var err = c.client.Get().
		Resource("connection").
		SubResource("list").
		Timeout(timeout).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *connection) Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true

	return c.client.Get().
		Resource("connection").
		SubResource("watch").
		Timeout(timeout).
		Watch(ctx)
}

func (c *connection) Patch(ctx context.Context, name string, pt string, data []byte, _ meta.PatchOptions, subresources ...string) (*meta.Connection, error) {
	var result = &meta.Connection{}
	var err = c.client.Patch(pt).
		Resource("connection").
		Name(name).
		SubResource(subresources...).
		Body(data).
		Do(ctx).
		Into(result)

	return result, err
}
