package registry

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"github.com/tsundata/flowline/pkg/runtime/serializer/json"
)

func NewRequestScope() *RequestScope {
	return &RequestScope{
		Serializer:          json.NewBasicNegotiatedSerializer(),
		StandardSerializers: nil,
		Resource:            schema.GroupVersionResource{},
		Kind:                schema.GroupVersionKind{},
		Subresource:         "",
		MetaGroupVersion:    schema.GroupVersion{},
		HubGroupVersion:     schema.GroupVersion{},
		MaxRequestBodyBytes: 0,
	}
}

// RequestScope encapsulates common fields across all RESTFul handler methods.
type RequestScope struct {
	Serializer runtime.NegotiatedSerializer

	// StandardSerializers, if set, restricts which serializers can be used when
	// we aren't transforming the output (into Table or PartialObjectMetadata).
	// Used only by CRDs which do not yet support Protobuf.
	StandardSerializers []runtime.SerializerInfo

	Resource schema.GroupVersionResource
	Kind     schema.GroupVersionKind

	Verb        string
	Subresource string

	MetaGroupVersion schema.GroupVersion

	// HubGroupVersion indicates what version objects read from etcd or incoming requests should be converted to for in-memory handling.
	HubGroupVersion schema.GroupVersion

	MaxRequestBodyBytes int64
}

func (scope *RequestScope) AllowsMediaTypeTransform(mimeType, mimeSubType string, gvk *schema.GroupVersionKind) bool {
	// some handlers like CRDs can't serve all the mime types that PartialObjectMetadata or Table can - if
	// gvk is nil (no conversion) allow StandardSerializers to further restrict the set of mime types.
	if gvk == nil {
		if len(scope.StandardSerializers) == 0 {
			return true
		}
		for _, info := range scope.StandardSerializers {
			if info.MediaTypeType == mimeType && info.MediaTypeSubType == mimeSubType {
				return true
			}
		}
		return false
	}

	return false
}

func (scope *RequestScope) AllowsServerVersion(version string) bool {
	return version == scope.MetaGroupVersion.Version
}

func (scope *RequestScope) AllowsStreamSchema(s string) bool {
	return s == "watch"
}
