package v1

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/watch"
	"time"
)

type WorkflowGetter interface {
	Workflow() WorkflowInterface
}

type WorkflowInterface interface {
	Create(ctx context.Context, workflow *meta.Workflow, opts meta.CreateOptions) (*meta.Workflow, error)
	Update(ctx context.Context, workflow *meta.Workflow, opts meta.UpdateOptions) (*meta.Workflow, error)
	UpdateStatus(ctx context.Context, workflow *meta.Workflow, opts meta.UpdateOptions) (*meta.Workflow, error)
	Delete(ctx context.Context, name string, opts meta.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error
	Get(ctx context.Context, name string, opts meta.GetOptions) (*meta.Workflow, error)
	List(ctx context.Context, opts meta.ListOptions) (*meta.WorkflowList, error)
	Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt string, data []byte, opts meta.PatchOptions, subresources ...string) (result *meta.Workflow, err error)

	WorkflowExpansion
}

// The WorkflowExpansion interface allows manually adding extra methods to the WorkflowInterface.
type WorkflowExpansion interface {
	Bind(ctx context.Context, binding *meta.Binding, opts meta.CreateOptions) error
	GetDag(ctx context.Context, name string, opts meta.GetOptions) (*meta.Dag, error)
}

type workflow struct {
	client rest.Interface
}

func newWorkflow(c *CoreV1Client) *workflow {
	return &workflow{client: c.RESTClient()}
}

func (c *workflow) Create(ctx context.Context, workflow *meta.Workflow, _ meta.CreateOptions) (*meta.Workflow, error) {
	var result = &meta.Workflow{}
	var err = c.client.Post().
		Resource("workflow").
		Body(workflow).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *workflow) Update(ctx context.Context, workflow *meta.Workflow, _ meta.UpdateOptions) (*meta.Workflow, error) {
	var result = &meta.Workflow{}
	var err = c.client.Put().
		Resource("workflow").
		Body(workflow).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *workflow) UpdateStatus(ctx context.Context, workflow *meta.Workflow, _ meta.UpdateOptions) (*meta.Workflow, error) {
	var result = &meta.Workflow{}
	var err = c.client.Put().
		Resource("workflow").
		Name(workflow.UID).
		SubResource("state").
		Body(workflow).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *workflow) Delete(ctx context.Context, name string, opts meta.DeleteOptions) error {
	return c.client.Delete().
		Resource("workflow").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *workflow) DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}

	return c.client.Delete().
		Resource("workflow").
		//VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *workflow) Get(ctx context.Context, name string, _ meta.GetOptions) (*meta.Workflow, error) {
	var result = &meta.Workflow{}
	err := c.client.Get().
		Resource("workflow").
		Name(name).
		//VersionedParams(&opts, nil)
		Do(ctx).
		Into(result)

	return result, err
}

func (c *workflow) List(ctx context.Context, opts meta.ListOptions) (*meta.WorkflowList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}

	var result = &meta.WorkflowList{}
	var err = c.client.Get().
		Resource("workflow").
		SubResource("list").
		Timeout(timeout).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *workflow) Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true

	return c.client.Get().
		Resource("workflow").
		SubResource("watch").
		Timeout(timeout).
		Watch(ctx)
}

func (c *workflow) Patch(ctx context.Context, name string, pt string, data []byte, _ meta.PatchOptions, subresources ...string) (*meta.Workflow, error) {
	var result = &meta.Workflow{}
	var err = c.client.Patch(pt).
		Resource("workflow").
		Name(name).
		SubResource(subresources...).
		//VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *workflow) Bind(ctx context.Context, binding *meta.Binding, _ meta.CreateOptions) error {
	return c.client.Post().
		Resource("workflow").
		Name(binding.UID).
		SubResource("binding").
		Body(binding).
		Do(ctx).
		Error()
}

func (c *workflow) GetDag(ctx context.Context, name string, _ meta.GetOptions) (*meta.Dag, error) {
	var result = &meta.Dag{}
	err := c.client.Get().
		Resource("workflow").
		Name(name).
		SubResource("dag").
		Do(ctx).
		Into(result)

	return result, err
}
