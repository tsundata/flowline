package informers

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/informer"
	"github.com/tsundata/flowline/pkg/informer/informers/core"
	"github.com/tsundata/flowline/pkg/informer/informers/internalinterfaces"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"reflect"
	"sync"
	"time"
)

type SharedInformerFactory interface {
	internalinterfaces.SharedInformerFactory
	ForResource(resource schema.GroupVersionResource) (GenericInformer, error)
	WaitForCacheSync(stopCh <-chan struct{}) map[reflect.Type]bool

	Core() core.Interface
}

type sharedInformerFactory struct {
	client           *client.RestClient
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	lock             sync.Mutex
	defaultResync    time.Duration
	customResync     map[reflect.Type]time.Duration

	informers        map[reflect.Type]informer.SharedIndexInformer
	startedInformers map[reflect.Type]bool
}

// SharedInformerOption defines the functional option type for SharedInformerFactory.
type SharedInformerOption func(*sharedInformerFactory) *sharedInformerFactory

// WithCustomResyncConfig sets a custom resync period for the specified informer types.
func WithCustomResyncConfig(resyncConfig map[meta.Object]time.Duration) SharedInformerOption {
	return func(factory *sharedInformerFactory) *sharedInformerFactory {
		for k, v := range resyncConfig {
			factory.customResync[reflect.TypeOf(k)] = v
		}
		return factory
	}
}

// WithTweakListOptions sets a custom filter on all listers of the configured SharedInformerFactory.
func WithTweakListOptions(tweakListOptions internalinterfaces.TweakListOptionsFunc) SharedInformerOption {
	return func(factory *sharedInformerFactory) *sharedInformerFactory {
		factory.tweakListOptions = tweakListOptions
		return factory
	}
}

// NewSharedInformerFactory constructs a new instance of sharedInformerFactory for all namespaces.
func NewSharedInformerFactory(client *client.RestClient, defaultResync time.Duration) SharedInformerFactory {
	return NewSharedInformerFactoryWithOptions(client, defaultResync)
}

// NewSharedInformerFactoryWithOptions constructs a new instance of a SharedInformerFactory with additional options.
func NewSharedInformerFactoryWithOptions(client *client.RestClient, defaultResync time.Duration, options ...SharedInformerOption) SharedInformerFactory {
	factory := &sharedInformerFactory{
		client:           client,
		defaultResync:    defaultResync,
		informers:        make(map[reflect.Type]informer.SharedIndexInformer),
		startedInformers: make(map[reflect.Type]bool),
		customResync:     make(map[reflect.Type]time.Duration),
	}

	// Apply all options
	for _, opt := range options {
		factory = opt(factory)
	}

	return factory
}

// Start initializes all requested informers.
func (f *sharedInformerFactory) Start(stopCh <-chan struct{}) {
	f.lock.Lock()
	defer f.lock.Unlock()

	for informerType, informer := range f.informers {
		if !f.startedInformers[informerType] {
			go informer.Run(stopCh)
			f.startedInformers[informerType] = true
		}
	}
}

// WaitForCacheSync waits for all started informers' cache were synced.
func (f *sharedInformerFactory) WaitForCacheSync(stopCh <-chan struct{}) map[reflect.Type]bool {
	is := func() map[reflect.Type]informer.SharedIndexInformer {
		f.lock.Lock()
		defer f.lock.Unlock()

		is := map[reflect.Type]informer.SharedIndexInformer{}
		for informerType, informer := range f.informers {
			if f.startedInformers[informerType] {
				is[informerType] = informer
			}
		}
		return is
	}()

	res := map[reflect.Type]bool{}
	for informType, ir := range is {
		res[informType] = informer.WaitForCacheSync(stopCh, ir.HasSynced)
	}
	return res
}

// InformerFor returns the SharedIndexInformer for obj using an internal client.
func (f *sharedInformerFactory) InformerFor(obj runtime.Object, newFunc internalinterfaces.NewInformerFunc) informer.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerType := reflect.TypeOf(obj)
	ir, exists := f.informers[informerType]
	if exists {
		return ir
	}

	resyncPeriod, exists := f.customResync[informerType]
	if !exists {
		resyncPeriod = f.defaultResync
	}

	ir = newFunc(f.client, resyncPeriod)
	f.informers[informerType] = ir

	return ir
}

func (f *sharedInformerFactory) ForResource(resource schema.GroupVersionResource) (GenericInformer, error) {
	switch resource {
	case schema.SchemeGroupVersion.WithResource("worker"):
		return &genericInformer{informer: f.Core().V1().Stages().Informer(), resource: resource.GroupResource()}, nil
	case schema.SchemeGroupVersion.WithResource("stages"):
		return &genericInformer{informer: f.Core().V1().Workers().Informer(), resource: resource.GroupResource()}, nil
	}

	return nil, fmt.Errorf("no informer found for %v", resource)
}

func (f *sharedInformerFactory) Core() core.Interface {
	return core.New(f, f.tweakListOptions)
}
