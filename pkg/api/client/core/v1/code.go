package v1

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/watch"
	"time"
)

type CodeGetter interface {
	Code() CodeInterface
}

type CodeInterface interface {
	Create(ctx context.Context, code *meta.Code, opts meta.CreateOptions) (*meta.Code, error)
	Update(ctx context.Context, code *meta.Code, opts meta.UpdateOptions) (*meta.Code, error)
	Delete(ctx context.Context, name string, opts meta.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error
	Get(ctx context.Context, name string, opts meta.GetOptions) (*meta.Code, error)
	List(ctx context.Context, opts meta.ListOptions) (*meta.CodeList, error)
	Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt string, data []byte, opts meta.PatchOptions, subresources ...string) (result *meta.Code, err error)
}

type code struct {
	client rest.Interface
}

func newCode(c *CoreV1Client) *code {
	return &code{client: c.RESTClient()}
}

func (c *code) Create(ctx context.Context, code *meta.Code, _ meta.CreateOptions) (*meta.Code, error) {
	var result = &meta.Code{}
	var err = c.client.Post().
		Resource("code").
		Body(code).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *code) Update(ctx context.Context, code *meta.Code, _ meta.UpdateOptions) (*meta.Code, error) {
	var result = &meta.Code{}
	var err = c.client.Put().
		Resource("code").
		Name(code.UID).
		Body(code).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *code) Delete(ctx context.Context, name string, opts meta.DeleteOptions) error {
	return c.client.Delete().
		Resource("code").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *code) DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}

	return c.client.Delete().
		Resource("code").
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *code) Get(ctx context.Context, name string, _ meta.GetOptions) (*meta.Code, error) {
	var result = &meta.Code{}
	err := c.client.Get().
		Resource("code").
		Name(name).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *code) List(ctx context.Context, opts meta.ListOptions) (*meta.CodeList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}

	var result = &meta.CodeList{}
	var err = c.client.Get().
		Resource("code").
		SubResource("list").
		Timeout(timeout).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *code) Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true

	return c.client.Get().
		Resource("code").
		SubResource("watch").
		Timeout(timeout).
		Watch(ctx)
}

func (c *code) Patch(ctx context.Context, name string, pt string, data []byte, _ meta.PatchOptions, subresources ...string) (*meta.Code, error) {
	var result = &meta.Code{}
	var err = c.client.Patch(pt).
		Resource("code").
		Name(name).
		SubResource(subresources...).
		Body(data).
		Do(ctx).
		Into(result)

	return result, err
}
