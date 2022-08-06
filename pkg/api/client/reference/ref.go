package reference

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"golang.org/x/xerrors"
)

var (
	// ErrNilObject Errors that could be returned by GetReference.
	ErrNilObject = xerrors.New("can't reference a nil object")
)

// GetReference returns an ObjectReference which refers to the given
// object, or an error if the object doesn't follow the conventions
// that would allow this.
func GetReference(scheme *runtime.Scheme, obj runtime.Object) (*meta.ObjectReference, error) {
	if obj == nil {
		return nil, ErrNilObject
	}
	if ref, ok := obj.(*meta.ObjectReference); ok {
		// Don't make a reference to a reference.
		return ref, nil
	}

	// An object that implements only List has enough metadata to build a reference
	var listMeta meta.Common
	objectMeta, err := meta.Accessor(obj)
	if err != nil {
		listMeta, err = meta.CommonAccessor(obj)
		if err != nil {
			return nil, err
		}
	} else {
		listMeta = objectMeta
	}

	gvk := obj.GetObjectKind().GroupVersionKind()

	// If object meta doesn't contain data about kind and/or version,
	// we are falling back to scheme.
	if gvk.Empty() {
		gvks, _, err := scheme.ObjectKinds(obj)
		if err != nil {
			return nil, err
		}
		if len(gvks) == 0 || gvks[0].Empty() {
			return nil, fmt.Errorf("unexpected gvks registered for object %T: %v", obj, gvks)
		}
		gvk = gvks[0]
	}

	kind := gvk.Kind
	version := gvk.GroupVersion().String()

	// only has list metadata
	if objectMeta == nil {
		return &meta.ObjectReference{
			Kind:            kind,
			APIVersion:      version,
			ResourceVersion: listMeta.GetResourceVersion(),
		}, nil
	}

	return &meta.ObjectReference{
		Kind:            kind,
		APIVersion:      version,
		Name:            objectMeta.GetName(),
		UID:             objectMeta.GetUID(),
		ResourceVersion: objectMeta.GetResourceVersion(),
	}, nil
}
