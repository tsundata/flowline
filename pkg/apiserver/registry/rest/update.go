package rest

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
)

type RESTUpdateStrategy interface {
}

// TransformFunc is a function to transform and return newObj
type TransformFunc func(ctx context.Context, newObj runtime.Object, oldObj runtime.Object) (transformedNewObj runtime.Object, err error)

// defaultUpdatedObjectInfo implements UpdatedObjectInfo
type defaultUpdatedObjectInfo struct {
	// obj is the updated object
	obj runtime.Object

	// transformers is an optional list of transforming functions that modify or
	// replace obj using information from the context, old object, or other sources.
	transformers []TransformFunc
}

// DefaultUpdatedObjectInfo returns an UpdatedObjectInfo impl based on the specified object.
func DefaultUpdatedObjectInfo(obj runtime.Object, transformers ...TransformFunc) UpdatedObjectInfo {
	return &defaultUpdatedObjectInfo{obj, transformers}
}

// Preconditions satisfies the UpdatedObjectInfo interface.
func (i *defaultUpdatedObjectInfo) Preconditions() *meta.Preconditions {
	// Attempt to get the UID out of the object
	accessor, err := meta.Accessor(i.obj)
	if err != nil {
		// If no UID can be read, no preconditions are possible
		return nil
	}

	// If empty, no preconditions needed
	uid := accessor.GetUID()
	if len(uid) == 0 {
		return nil
	}

	return &meta.Preconditions{UID: &uid}
}

// UpdatedObject satisfies the UpdatedObjectInfo interface.
// It returns a copy of the held obj, passed through any configured transformers.
func (i *defaultUpdatedObjectInfo) UpdatedObject(ctx context.Context, oldObj runtime.Object) (runtime.Object, error) {
	var err error
	// Start with the configured object
	newObj := i.obj

	// If the original is non-nil (might be nil if the first transformer builds the object from the oldObj), make a copy,
	// so we don't return the original. BeforeUpdate can mutate the returned object, doing things like clearing ResourceVersion.
	// If we're re-called, we need to be able to return the pristine version.
	if newObj != nil {
		newObj = newObj.DeepCopyObject()
	}

	// Allow any configured transformers to update the new object
	for _, transformer := range i.transformers {
		newObj, err = transformer(ctx, newObj, oldObj)
		if err != nil {
			return nil, err
		}
	}

	return newObj, nil
}
