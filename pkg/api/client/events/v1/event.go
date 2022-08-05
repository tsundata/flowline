package v1

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/watch"
	"time"
)

type EventGetter interface {
	Event() EventInterface
}

type EventInterface interface {
	Create(ctx context.Context, event *meta.Event, opts meta.CreateOptions) (*meta.Event, error)
	Update(ctx context.Context, event *meta.Event, opts meta.UpdateOptions) (*meta.Event, error)
	Delete(ctx context.Context, name string, opts meta.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error
	Get(ctx context.Context, name string, opts meta.GetOptions) (*meta.Event, error)
	List(ctx context.Context, opts meta.ListOptions) (*meta.EventList, error)
	Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt string, data []byte, opts meta.PatchOptions, subresources ...string) (result *meta.Event, err error)
}

type event struct {
	client rest.Interface
}

func newEvent(c *EventsV1Client) *event {
	return &event{client: c.RESTClient()}
}

func (c *event) Create(ctx context.Context, event *meta.Event, _ meta.CreateOptions) (*meta.Event, error) {
	var result = &meta.Event{}
	var err = c.client.Post().
		Resource("event").
		Body(event).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *event) Update(ctx context.Context, event *meta.Event, _ meta.UpdateOptions) (*meta.Event, error) {
	var result = &meta.Event{}
	var err = c.client.Put().
		Resource("event").
		Name(event.UID).
		Body(event).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *event) Delete(ctx context.Context, name string, opts meta.DeleteOptions) error {
	return c.client.Delete().
		Resource("event").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *event) DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}

	return c.client.Delete().
		Resource("event").
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *event) Get(ctx context.Context, name string, _ meta.GetOptions) (*meta.Event, error) {
	var result = &meta.Event{}
	err := c.client.Get().
		Resource("event").
		Name(name).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *event) List(ctx context.Context, opts meta.ListOptions) (*meta.EventList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}

	var result = &meta.EventList{}
	var err = c.client.Get().
		Resource("event").
		SubResource("list").
		Timeout(timeout).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *event) Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true

	return c.client.Get().
		Resource("event").
		SubResource("watch").
		Timeout(timeout).
		Watch(ctx)
}

func (c *event) Patch(ctx context.Context, name string, pt string, data []byte, _ meta.PatchOptions, subresources ...string) (*meta.Event, error) {
	var result = &meta.Event{}
	var err = c.client.Patch(pt).
		Resource("event").
		Name(name).
		SubResource(subresources...).
		Body(data).
		Do(ctx).
		Into(result)

	return result, err
}
