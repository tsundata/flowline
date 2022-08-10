package rest

import (
	"context"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/warning"
	"github.com/tsundata/flowline/pkg/runtime"
	"golang.org/x/xerrors"
	"strings"
)

// RESTUpdateStrategy defines the minimum validation, accepted input, and
// name generation behavior to update an object that follows Kubernetes
// API conventions. A resource may have many UpdateStrategies, depending on
// the call pattern in use.
type RESTUpdateStrategy interface {
	runtime.ObjectTyper
	// AllowCreateOnUpdate returns true if the object can be created by a PUT.
	AllowCreateOnUpdate() bool
	// PrepareForUpdate is invoked on update before validation to normalize
	// the object.  For example: remove fields that are not to be persisted,
	// sort order-insensitive list fields, etc.  This should not remove fields
	// whose presence would be considered a validation error.
	PrepareForUpdate(ctx context.Context, obj, old runtime.Object)
	// ValidateUpdate is invoked after default fields in the object have been
	// filled in before the object is persisted.  This method should not mutate
	// the object.
	ValidateUpdate(ctx context.Context, obj, old runtime.Object) []error
	// WarningsOnUpdate returns warnings to the client performing the update.
	// WarningsOnUpdate is invoked after default fields in the object have been filled in
	// and after ValidateUpdate has passed, before Canonicalize is called, and before the object is persisted.
	// This method must not mutate either object.
	//
	// Be brief; limit warnings to 120 characters if possible.
	// Don't include a "Warning:" prefix in the message (that is added by clients on output).
	// Warnings returned about a specific field should be formatted as "path.to.field: message".
	// For example: `spec.imagePullSecrets[0].name: invalid empty name ""`
	//
	// Use warning messages to describe problems the client making the API request should correct or be aware of.
	// For example:
	// - use of deprecated fields/labels/annotations that will stop working in a future release
	// - use of obsolete fields/labels/annotations that are non-functional
	// - malformed or invalid specifications that prevent successful handling of the submitted object,
	//   but are not rejected by validation for compatibility reasons
	//
	// Warnings should not be returned for fields which cannot be resolved by the caller.
	// For example, do not warn about spec fields in a status update.
	WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string
	// Canonicalize allows an object to be mutated into a canonical form. This
	// ensures that code that operates on these objects can rely on the common
	// form for things like comparison.  Canonicalize is invoked after
	// validation has succeeded but before the object has been persisted.
	// This method may mutate the object.
	Canonicalize(obj runtime.Object)
	// AllowUnconditionalUpdate returns true if the object can be updated
	// unconditionally (irrespective of the latest resource version), when
	// there is no resource version specified in the object.
	AllowUnconditionalUpdate() bool
}

// BeforeUpdate ensures that common operations for all resources are performed on update. It only returns
// errors that can be converted to api.Status. It will invoke update validation with the provided existing
// and updated objects.
// It sets zero values only if the object does not have a zero value for the respective field.
func BeforeUpdate(strategy RESTUpdateStrategy, ctx context.Context, obj, old runtime.Object) error {
	objectMeta, kind, kerr := objectMetaAndKind(strategy, obj)
	if kerr != nil {
		return kerr
	}

	// Ensure requests cannot update generation
	oldMeta, err := meta.Accessor(old)
	if err != nil {
		return err
	}
	objectMeta.SetGeneration(oldMeta.GetGeneration())

	strategy.PrepareForUpdate(ctx, obj, old)

	// Use the existing UID if none is provided
	if len(objectMeta.GetUID()) == 0 {
		objectMeta.SetUID(oldMeta.GetUID())
	}
	// ignore changes to timestamp
	if oldCreationTime := oldMeta.GetCreationTimestamp(); !oldCreationTime.IsZero() {
		objectMeta.SetCreationTimestamp(oldMeta.GetCreationTimestamp())
	}
	// an update can never remove/change a deletion timestamp
	if !oldMeta.GetDeletionTimestamp().IsZero() {
		objectMeta.SetDeletionTimestamp(oldMeta.GetDeletionTimestamp())
	}
	// an update can never remove/change grace period seconds
	if oldMeta.GetDeletionGracePeriodSeconds() != nil && objectMeta.GetDeletionGracePeriodSeconds() == nil {
		objectMeta.SetDeletionGracePeriodSeconds(oldMeta.GetDeletionGracePeriodSeconds())
	}

	errs := strategy.ValidateUpdate(ctx, obj, old)
	if len(errs) > 0 {
		errStr := strings.Builder{}
		for _, err := range errs {
			errStr.WriteString(err.Error())
		}
		return xerrors.Errorf("%s %s %s", kind.GroupKind(), objectMeta.GetName(), errStr)
	}

	for _, w := range strategy.WarningsOnUpdate(ctx, obj, old) {
		warning.AddWarning(ctx, "", w)
	}

	strategy.Canonicalize(obj)

	return nil
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
