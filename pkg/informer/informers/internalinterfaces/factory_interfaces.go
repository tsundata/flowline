package internalinterfaces

import (
	"github.com/tsundata/flowline/pkg/api/client"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/informer"
	"github.com/tsundata/flowline/pkg/runtime"
	"time"
)

// NewInformerFunc takes client.Interface and time.Duration to return a SharedIndexInformer.
type NewInformerFunc func(client.Interface, time.Duration) informer.SharedIndexInformer

// SharedInformerFactory a small interface to allow for adding an informer without an import cycle
type SharedInformerFactory interface {
	Start(stopCh <-chan struct{})
	InformerFor(obj runtime.Object, newFunc NewInformerFunc) informer.SharedIndexInformer
}

// TweakListOptionsFunc is a function that transforms a v1.ListOptions.
type TweakListOptionsFunc func(*meta.ListOptions)
