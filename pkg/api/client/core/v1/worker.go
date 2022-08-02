package v1

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/watch"
	"time"
)

type WorkerGetter interface {
	Worker() WorkerInterface
}

type WorkerInterface interface {
	Create(ctx context.Context, worker *meta.Worker, opts meta.CreateOptions) (*meta.Worker, error)
	Update(ctx context.Context, worker *meta.Worker, opts meta.UpdateOptions) (*meta.Worker, error)
	UpdateStatus(ctx context.Context, worker *meta.Worker, opts meta.UpdateOptions) (*meta.Worker, error)
	Delete(ctx context.Context, name string, opts meta.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error
	Get(ctx context.Context, name string, opts meta.GetOptions) (*meta.Worker, error)
	List(ctx context.Context, opts meta.ListOptions) (*meta.WorkerList, error)
	Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt string, data []byte, opts meta.PatchOptions, subresources ...string) (result *meta.Worker, err error)

	WorkerExpansion
}

// The WorkerExpansion interface allows manually adding extra methods to the WorkerInterface.
type WorkerExpansion interface {
	Bind(ctx context.Context, binding *meta.Binding, opts meta.CreateOptions) error
	Heartbeat(ctx context.Context, worker *meta.Worker, opts meta.UpdateOptions) (*meta.Status, error)
	Register(ctx context.Context, worker *meta.Worker, opts meta.CreateOptions) (*meta.Worker, error)
}

type worker struct {
	client rest.Interface
}

func newWorker(c *CoreV1Client) *worker {
	return &worker{client: c.RESTClient()}
}

func (c *worker) Create(ctx context.Context, worker *meta.Worker, _ meta.CreateOptions) (*meta.Worker, error) {
	var result = &meta.Worker{}
	var err = c.client.Post().
		Resource("worker").
		Body(worker).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *worker) Update(ctx context.Context, worker *meta.Worker, _ meta.UpdateOptions) (*meta.Worker, error) {
	var result = &meta.Worker{}
	var err = c.client.Put().
		Resource("worker").
		Body(worker).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *worker) UpdateStatus(ctx context.Context, worker *meta.Worker, _ meta.UpdateOptions) (*meta.Worker, error) {
	var result = &meta.Worker{}
	var err = c.client.Put().
		Resource("worker").
		Name(worker.UID).
		SubResource("state").
		Body(worker).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *worker) Delete(ctx context.Context, name string, opts meta.DeleteOptions) error {
	return c.client.Delete().
		Resource("worker").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *worker) DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}

	return c.client.Delete().
		Resource("worker").
		//VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *worker) Get(ctx context.Context, name string, _ meta.GetOptions) (*meta.Worker, error) {
	var result = &meta.Worker{}
	err := c.client.Get().
		Resource("worker").
		Name(name).
		//VersionedParams(&opts, nil)
		Do(ctx).
		Into(result)

	return result, err
}

func (c *worker) List(ctx context.Context, opts meta.ListOptions) (*meta.WorkerList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}

	var result = &meta.WorkerList{}
	var err = c.client.Get().
		Resource("worker").
		SubResource("list").
		Timeout(timeout).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *worker) Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true

	return c.client.Get().
		Resource("worker").
		SubResource("watch").
		Timeout(timeout).
		Watch(ctx)
}

func (c *worker) Patch(ctx context.Context, name string, pt string, data []byte, _ meta.PatchOptions, subresources ...string) (*meta.Worker, error) {
	var result = &meta.Worker{}
	var err = c.client.Patch(pt).
		Resource("nodes").
		Name(name).
		SubResource(subresources...).
		//VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *worker) Bind(ctx context.Context, binding *meta.Binding, _ meta.CreateOptions) error {
	return c.client.Post().
		Resource("worker").
		Name(binding.UID).
		SubResource("binding").
		Body(binding).
		Do(ctx).
		Error()
}

func (c *worker) Register(ctx context.Context, worker *meta.Worker, _ meta.CreateOptions) (*meta.Worker, error) {
	var result = &meta.Worker{}
	var err = c.client.Post().
		Resource("worker").
		SubResource("register").
		Body(worker).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *worker) Heartbeat(ctx context.Context, worker *meta.Worker, _ meta.UpdateOptions) (*meta.Status, error) {
	var result = &meta.Status{}
	var err = c.client.Put().
		Resource("worker").
		Name(worker.UID).
		SubResource("heartbeat").
		Body(worker).
		Do(ctx).
		Into(result)

	return result, err
}
