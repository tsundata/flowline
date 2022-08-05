package handlers

import (
	"context"
	"errors"
	"fmt"
	"github.com/emicklei/go-restful/v3"
	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/apiserver/registry"
	"github.com/tsundata/flowline/pkg/apiserver/registry/negotiation"
	"github.com/tsundata/flowline/pkg/apiserver/registry/rest"
	"github.com/tsundata/flowline/pkg/apiserver/storage"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"net/http"
	"strings"
)

const (
	// maximum number of operations a single json patch may contain.
	maxJSONPatchOperations = 10000
)

func patchHandler(r rest.Patcher, scope *registry.RequestScope) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		subResource, isSubResource := r.(rest.SubResourceStorage)
		if isSubResource && scope.Subresource != "" {
			subResource.Handle(scope.Verb, scope.Subresource, req, resp)
			return
		}

		uid := req.PathParameter("uid")
		contentType := req.Request.Header.Get("Content-Type")
		// Remove "; charset=" if included in header.
		if idx := strings.Index(contentType, ";"); idx > 0 {
			contentType = contentType[:idx]
		}
		patchType := meta.PatchType(contentType)

		// enforce a timeout of at most requestTimeoutUpperBound (34s) or less if the user-provided
		// timeout inside the parent context is lower than requestTimeoutUpperBound.
		ctx, cancel := context.WithTimeout(req.Request.Context(), requestTimeoutUpperBound)
		defer cancel()

		_, _, err := negotiation.NegotiateOutputMediaType(req.Request, scope.Serializer, scope)
		if err != nil {
			_ = resp.WriteError(http.StatusInternalServerError, err)
			return
		}

		patchBytes, err := limitedReadBody(req.Request, scope.MaxRequestBodyBytes)
		if err != nil {
			_ = resp.WriteError(http.StatusInternalServerError, err)
			return
		}

		baseContentType := runtime.ContentTypeJSON
		s, ok := runtime.SerializerInfoForMediaType(scope.Serializer.SupportedMediaTypes(), baseContentType)
		if !ok {
			_ = resp.WriteError(http.StatusInternalServerError, fmt.Errorf("no serializer defined for %v", baseContentType))
			return
		}

		gv := scope.Kind.GroupVersion()

		decodeSerializer := s.Serializer

		codec := runtime.NewCodec(
			scope.Serializer.EncoderForVersion(s.Serializer, gv),
			scope.Serializer.DecoderToVersion(decodeSerializer, scope.HubGroupVersion),
		)

		options := &meta.PatchOptions{}
		options.TypeMeta.SetGroupVersionKind(rest.SchemeGroupVersion.WithKind("PatchOptions"))

		p := patcher{
			resource: scope.Resource,
			kind:     scope.Kind,

			hubGroupVersion: scope.HubGroupVersion,

			createValidation: rest.ValidateAllObjectFunc,
			updateValidation: rest.ValidateAllObjectUpdateFunc,

			forceAllowCreate: true,

			codec: codec,

			options: options,

			restPatcher: r,
			name:        uid,
			patchType:   patchType,
			patchBytes:  patchBytes,
			userAgent:   req.Request.UserAgent(),
		}

		result, _, err := p.patchResource(ctx, scope)
		if err != nil {
			_ = resp.WriteError(http.StatusInternalServerError, err)
			return
		}

		_ = resp.WriteEntity(result)
	}
}

func PatchResource(s rest.Patcher, scope *registry.RequestScope) restful.RouteFunction {
	return patchHandler(s, scope)
}

type patcher struct {
	// Pieces of RequestScope
	resource            schema.GroupVersionResource
	kind                schema.GroupVersionKind
	subresource         string
	dryRun              bool
	validationDirective string

	hubGroupVersion schema.GroupVersion

	// Validation functions
	createValidation rest.ValidateObjectFunc
	updateValidation rest.ValidateObjectUpdateFunc

	codec runtime.Codec

	options *meta.PatchOptions

	// Operation information
	restPatcher rest.Patcher
	name        string
	patchType   meta.PatchType
	patchBytes  []byte
	userAgent   string

	// Set at invocation-time (by applyPatch) and immutable thereafter
	updatedObjectInfo rest.UpdatedObjectInfo
	mechanism         patchMechanism
	forceAllowCreate  bool
}

func (p *patcher) patchResource(ctx context.Context, scope *registry.RequestScope) (runtime.Object, bool, error) {
	switch p.patchType {
	case meta.JSONPatchType, meta.MergePatchType:
		p.mechanism = &jsonPatcher{
			patcher:      p,
			fieldManager: nil, // todo
		}
	default:
		return nil, false, fmt.Errorf("%s: unimplemented patch type", p.patchType)
	}

	wasCreated := false
	p.updatedObjectInfo = rest.DefaultUpdatedObjectInfo(nil, p.applyPatch)
	requestFunc := func() (runtime.Object, error) {
		options := patchToUpdateOptions(p.options)

		find, err := p.restPatcher.Get(ctx, p.name, &meta.GetOptions{})
		if err != nil && !errors.Is(err, storage.ErrKeyNotFound) {
			return nil, err
		}
		if errors.Is(err, storage.ErrKeyNotFound) {
			// return nil, err fixme
		}
		patchedObj, err := p.updatedObjectInfo.UpdatedObject(ctx, find)
		if err != nil {
			return nil, err
		}

		updateObject, created, updateErr := p.restPatcher.Update(ctx, p.name, patchedObj, p.createValidation, p.updateValidation, p.forceAllowCreate, options)
		wasCreated = created
		return updateObject, updateErr
	}

	result, err := requestFunc()
	return result, wasCreated, err
}

// patchToUpdateOptions creates an UpdateOptions with the same field values as the provided PatchOptions.
func patchToUpdateOptions(po *meta.PatchOptions) *meta.UpdateOptions {
	if po == nil {
		return nil
	}
	uo := &meta.UpdateOptions{
		DryRun:          po.DryRun,
		FieldManager:    po.FieldManager,
		FieldValidation: po.FieldValidation,
	}
	uo.TypeMeta.SetGroupVersionKind(rest.SchemeGroupVersion.WithKind("UpdateOptions"))
	return uo
}

// applyPatch is called every time GuaranteedUpdate asks for the updated object,
// and is given the currently persisted object as input.
func (p *patcher) applyPatch(ctx context.Context, _, currentObject runtime.Object) (objToUpdate runtime.Object, patchErr error) {
	// Make sure we actually have a persisted currentObject
	currentObjectHasUID, err := hasUID(currentObject)
	if err != nil {
		return nil, err
	} else if !currentObjectHasUID {
		objToUpdate, patchErr = p.mechanism.createNewObject(ctx)
	} else {
		objToUpdate, patchErr = p.mechanism.applyPatchToCurrentObject(ctx, currentObject)
	}

	if patchErr != nil {
		return nil, patchErr
	}

	objToUpdateHasUID, err := hasUID(objToUpdate)
	if err != nil {
		return nil, err
	}
	if objToUpdateHasUID && !currentObjectHasUID {
		accessor, err := meta.Accessor(objToUpdate)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("uid mismatch: the provided object specified uid %s, and no existing object was found", accessor.GetUID())
	}

	return objToUpdate, nil
}

type patchMechanism interface {
	applyPatchToCurrentObject(requextContext context.Context, currentObject runtime.Object) (runtime.Object, error)
	createNewObject(requestContext context.Context) (runtime.Object, error)
}

type jsonPatcher struct {
	*patcher

	fieldManager interface{}
}

func (p *jsonPatcher) applyPatchToCurrentObject(_ context.Context, currentObject runtime.Object) (runtime.Object, error) {
	// Encode will convert & return a versioned object in JSON.
	currentObjJS, err := runtime.Encode(p.codec, currentObject)
	if err != nil {
		return nil, err
	}

	// Apply the patch.
	patchedObjJS, appliedStrictErrs, err := p.applyJSPatch(currentObjJS)
	if err != nil {
		return nil, err
	}

	// Construct the resulting typed, unversioned object.
	objToUpdate := p.restPatcher.New()
	if err := runtime.DecodeInto(p.codec, patchedObjJS, objToUpdate); err != nil {
		return nil, err
	} else if len(appliedStrictErrs) > 0 {
		return nil, fmt.Errorf("StrictDecodingError %v", appliedStrictErrs)
	}

	if p.fieldManager != nil {
		// objToUpdate = p.fieldManager.UpdateNoErrors(currentObject, objToUpdate, managerOrUserAgent(p.options.FieldManager, p.userAgent))
	}
	return objToUpdate, nil
}

func (p *jsonPatcher) createNewObject(_ context.Context) (runtime.Object, error) {
	return nil, fmt.Errorf("NotFound %s %s", p.resource.GroupResource(), p.name)
}

// applyJSPatch applies the patch. Input and output objects must both have
// the external version, since that is what the patch must have been constructed against.
func (p *jsonPatcher) applyJSPatch(versionedJS []byte) (patchedJS []byte, strictErrors []error, retErr error) {
	switch p.patchType {
	case meta.JSONPatchType:
		patchObj, err := jsonpatch.DecodePatch(p.patchBytes)
		if err != nil {
			return nil, nil, err
		}
		if len(patchObj) > maxJSONPatchOperations {
			return nil, nil, fmt.Errorf("the allowed maximum operations in a JSON patch is %d, got %d",
				maxJSONPatchOperations, len(patchObj))
		}
		patchedJS, err := patchObj.Apply(versionedJS)
		if err != nil {
			return nil, nil, err
		}
		return patchedJS, strictErrors, nil
	case meta.MergePatchType:
		patchedJS, retErr = jsonpatch.MergePatch(versionedJS, p.patchBytes)
		if retErr != nil {
			return nil, nil, retErr
		}
		return patchedJS, strictErrors, retErr
	default:
		// only here as a safety net - go-restful filters content-type
		return nil, nil, fmt.Errorf("unknown Content-Type header for patch: %v", p.patchType)
	}
}
