package v1

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/watch"
	"time"
)

type JobGetter interface {
	Job() JobInterface
}

type JobInterface interface {
	Create(ctx context.Context, pod *meta.Job, opts meta.CreateOptions) (*meta.Job, error)
	Update(ctx context.Context, pod *meta.Job, opts meta.UpdateOptions) (*meta.Job, error)
	UpdateStatus(ctx context.Context, pod *meta.Job, opts meta.UpdateOptions) (*meta.Job, error)
	Delete(ctx context.Context, name string, opts meta.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error
	Get(ctx context.Context, name string, opts meta.GetOptions) (*meta.Job, error)
	List(ctx context.Context, opts meta.ListOptions) (*meta.JobList, error)
	Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt string, data []byte, opts meta.PatchOptions, subresources ...string) (result *meta.Job, err error)

	JobExpansion
}

// The JobExpansion interface allows manually adding extra methods to the PodInterface.
type JobExpansion interface {
	Bind(ctx context.Context, binding *meta.Binding, opts meta.CreateOptions) error
}

type job struct {
	client rest.Interface
}

func newJob(c *CoreV1Client) *job {
	return &job{client: c.RESTClient()}
}

func (c *job) Create(ctx context.Context, job *meta.Job, _ meta.CreateOptions) (*meta.Job, error) {
	var result = &meta.Job{}
	var err = c.client.Post().
		Resource("job").
		Body(job).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *job) Update(ctx context.Context, job *meta.Job, _ meta.UpdateOptions) (*meta.Job, error) {
	var result = &meta.Job{}
	var err = c.client.Put().
		Resource("job").
		Body(job).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *job) UpdateStatus(ctx context.Context, job *meta.Job, _ meta.UpdateOptions) (*meta.Job, error) {
	var result = &meta.Job{}
	var err = c.client.Put().
		Resource("job").
		Name(job.UID).
		SubResource("state").
		Body(job).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *job) Delete(ctx context.Context, name string, opts meta.DeleteOptions) error {
	return c.client.Delete().
		Resource("job").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *job) DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}

	return c.client.Delete().
		Resource("job").
		//VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *job) Get(ctx context.Context, name string, _ meta.GetOptions) (*meta.Job, error) {
	var result = &meta.Job{}
	err := c.client.Get().
		Resource("job").
		Name(name).
		//VersionedParams(&opts, nil)
		Do(ctx).
		Into(result)

	return result, err
}

func (c *job) List(ctx context.Context, opts meta.ListOptions) (*meta.JobList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}

	var result = &meta.JobList{}
	var err = c.client.Get().
		Resource("job").
		SubResource("list").
		Timeout(timeout).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *job) Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true

	return c.client.Get().
		Resource("job").
		SubResource("watch").
		Timeout(timeout).
		Watch(ctx)
}

func (c *job) Patch(ctx context.Context, name string, pt string, data []byte, _ meta.PatchOptions, subresources ...string) (*meta.Job, error) {
	var result = &meta.Job{}
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

func (c *job) Bind(ctx context.Context, binding *meta.Binding, _ meta.CreateOptions) error {
	return c.client.Post().
		Resource("job").
		Name(binding.UID).
		SubResource("binding").
		Body(binding).
		Do(ctx).
		Error()
}
