package v1

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/client/rest"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/watch"
	"time"
)

type StagesGetter interface {
	Stages() StageInterface
}

type StageInterface interface {
	Create(ctx context.Context, pod *meta.Stage, opts meta.CreateOptions) (*meta.Stage, error)
	Update(ctx context.Context, pod *meta.Stage, opts meta.UpdateOptions) (*meta.Stage, error)
	UpdateStatus(ctx context.Context, pod *meta.Stage, opts meta.UpdateOptions) (*meta.Stage, error)
	Delete(ctx context.Context, name string, opts meta.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error
	Get(ctx context.Context, name string, opts meta.GetOptions) (*meta.Stage, error)
	List(ctx context.Context, opts meta.ListOptions) (*meta.StageList, error)
	Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt string, data []byte, opts meta.PatchOptions, subresources ...string) (result *meta.Stage, err error)

	StageExpansion
}

// The StageExpansion interface allows manually adding extra methods to the PodInterface.
type StageExpansion interface {
	Bind(ctx context.Context, binding *meta.Binding, opts meta.CreateOptions) error
}

type stages struct {
	client rest.Interface
}

func newStages(c *CoreV1Client) *stages {
	return &stages{client: c.RESTClient()}
}

func (c *stages) Create(ctx context.Context, stage *meta.Stage, _ meta.CreateOptions) (*meta.Stage, error) {
	var result = &meta.Stage{}
	var err = c.client.Post().
		Resource("stage").
		Body(stage).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *stages) Update(ctx context.Context, stage *meta.Stage, _ meta.UpdateOptions) (*meta.Stage, error) {
	var result = &meta.Stage{}
	var err = c.client.Put().
		Resource("stage").
		Body(stage).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *stages) UpdateStatus(ctx context.Context, stage *meta.Stage, _ meta.UpdateOptions) (*meta.Stage, error) {
	var result = &meta.Stage{}
	var err = c.client.Put().
		Resource("stage").
		Name(stage.Name).
		SubResource("state").
		Body(stage).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *stages) Delete(ctx context.Context, name string, opts meta.DeleteOptions) error {
	return c.client.Delete().
		Resource("stage").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *stages) DeleteCollection(ctx context.Context, opts meta.DeleteOptions, listOpts meta.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}

	return c.client.Delete().
		Resource("stage").
		//VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *stages) Get(ctx context.Context, name string, _ meta.GetOptions) (*meta.Stage, error) {
	var result = &meta.Stage{}
	err := c.client.Get().
		Resource("stage").
		Name(name).
		//VersionedParams(&opts, nil)
		Do(ctx).
		Into(result)

	return result, err
}

func (c *stages) List(ctx context.Context, opts meta.ListOptions) (*meta.StageList, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}

	var result = &meta.StageList{}
	var err = c.client.Get().
		Resource("stage").
		Timeout(timeout).
		Do(ctx).
		Into(result)

	return result, err
}

func (c *stages) Watch(ctx context.Context, opts meta.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true

	return c.client.Get().
		Resource("stage").
		Timeout(timeout).
		Watch(ctx)
}

func (c *stages) Patch(ctx context.Context, name string, pt string, data []byte, _ meta.PatchOptions, subresources ...string) (*meta.Stage, error) {
	var result = &meta.Stage{}
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

func (c *stages) Bind(ctx context.Context, binding *meta.Binding, _ meta.CreateOptions) error {
	return c.client.Post().
		Resource("stage").
		Name(binding.Name).
		SubResource("binding").
		Body(binding).
		Do(ctx).
		Error()
}
