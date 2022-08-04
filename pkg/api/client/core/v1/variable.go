package v1

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/watch"
	"time"
)

type VariableGetter interface {
	Variable() VariableInterface
}

type VariableInterface interface {
	Create(ctx context.Context, variable *meta.Variable, opts meta.CreateOptions) (*meta.Variable, error)
	Update(ctx context.Context, variable *meta.Variable, opts meta.UpdateOptions) (*meta.Variable, error)
	Delete(ctx context.Context, name string, opts meta.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error
	Get(ctx context.Context, name string, opts meta.GetOptions) (*meta.Variable, error)
	List(ctx context.Context, opts meta.ListOptions) (*meta.VariableList, error)
	Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt string, data []byte, opts meta.PatchOptions, subresources ...string) (result *meta.Variable, err error)
}

type variable struct {
	client rest.Interface
}

func newVariable(c *CoreV1Client) *variable {
	return &variable{client: c.RESTClient()}
}

func (c *variable) Create(ctx context.Context, variable *meta.Variable, _ meta.CreateOptions) (*meta.Variable, error) {
	var result = &meta.Variable{}
	var err = c.client.Post().
		Resource("variable").
		Body(variable).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *variable) Update(ctx context.Context, variable *meta.Variable, _ meta.UpdateOptions) (*meta.Variable, error) {
	var result = &meta.Variable{}
	var err = c.client.Put().
		Resource("variable").
		Name(variable.UID).
		Body(variable).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *variable) Delete(ctx context.Context, name string, opts meta.DeleteOptions) error {
	return c.client.Delete().
		Resource("variable").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *variable) DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}

	return c.client.Delete().
		Resource("variable").
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *variable) Get(ctx context.Context, name string, _ meta.GetOptions) (*meta.Variable, error) {
	var result = &meta.Variable{}
	err := c.client.Get().
		Resource("variable").
		Name(name).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *variable) List(ctx context.Context, opts meta.ListOptions) (*meta.VariableList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}

	var result = &meta.VariableList{}
	var err = c.client.Get().
		Resource("variable").
		SubResource("list").
		Timeout(timeout).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *variable) Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true

	return c.client.Get().
		Resource("variable").
		SubResource("watch").
		Timeout(timeout).
		Watch(ctx)
}

func (c *variable) Patch(ctx context.Context, name string, pt string, data []byte, _ meta.PatchOptions, subresources ...string) (*meta.Variable, error) {
	var result = &meta.Variable{}
	var err = c.client.Patch(pt).
		Resource("variable").
		Name(name).
		SubResource(subresources...).
		Body(data).
		Do(ctx).
		Into(result)

	return result, err
}
