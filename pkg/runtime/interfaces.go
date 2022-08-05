package runtime

import (
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"io"
	"net/url"
)

const (
	// APIVersionInternal may be used if you are registering a type that should not
	// be considered stable or serialized - it is a convention only and has no
	// special behavior in this package.
	APIVersionInternal = "v1"
)

// Object interface must be supported by all API types registered with Scheme. Since objects in a scheme are
// expected to be serialized to the wire, the interface an Object must provide to the Scheme allows
// serializers to set the kind, version, and group the object is represented as. An Object may choose
// to return a no-op ObjectKindAccessor in cases where it is not expected to be serialized.
type Object interface {
	GetObjectKind() schema.ObjectKind
	DeepCopyObject() Object
}

// GroupVersioner refines a set of possible conversion targets into a single option.
type GroupVersioner interface {
	// KindForGroupVersionKinds returns a desired target group version kind for the given input, or returns ok false if no
	// target is known. In general, if the return target is not in the input list, the caller is expected to invoke
	// Scheme.New(target) and then perform a conversion between the current Go type and the destination Go type.
	// Sophisticated implementations may use additional information about the input kinds to pick a destination kind.
	KindForGroupVersionKinds(kinds []schema.GroupVersionKind) (target schema.GroupVersionKind, ok bool)
	// Identifier returns string representation of the object.
	// Identifiers of two different encoders should be equal only if for every input
	// kinds they return the same result.
	Identifier() string
}

// Framer is a factory for creating readers and writers that obey a particular framing pattern.
type Framer interface {
	NewFrameReader(r io.ReadCloser) io.ReadCloser
	NewFrameWriter(w io.Writer) io.Writer
}

// MemoryAllocator is responsible for allocating memory.
// By encapsulating memory allocation into its own interface, we can reuse the memory
// across many operations in places we know it can significantly improve the performance.
type MemoryAllocator interface {
	// Allocate reserves memory for n bytes.
	// Note that implementations of this method are not required to zero the returned array.
	// It is the caller's responsibility to clean the memory if needed.
	Allocate(n uint64) []byte
}

// EncoderWithAllocator  serializes objects in a way that allows callers to manage any additional memory allocations.
type EncoderWithAllocator interface {
	Encoder
	// EncodeWithAllocator writes an object to a stream as Encode does.
	// In addition, it allows for providing a memory allocator for efficient memory usage during object serialization
	EncodeWithAllocator(obj Object, w io.Writer, memAlloc MemoryAllocator) error
}

// SerializerInfo contains information about a specific serialization format
type SerializerInfo struct {
	// MediaType is the value that represents this serializer over the wire.
	MediaType string
	// MediaTypeType is the first part of the MediaType ("application" in "application/json").
	MediaTypeType string
	// MediaTypeSubType is the second part of the MediaType ("json" in "application/json").
	MediaTypeSubType string
	// EncodesAsText indicates this serializer can be encoded to UTF-8 safely.
	EncodesAsText bool
	// Serializer is the individual object serializer for this media type.
	Serializer Serializer
	// PrettySerializer, if set, can serialize this object in a form biased towards
	// readability.
	PrettySerializer Serializer
	// StrictSerializer, if set, deserializes this object strictly,
	// erring on unknown fields.
	StrictSerializer Serializer
	// StreamSerializer, if set, describes the streaming serialization format
	// for this media type.
	StreamSerializer *StreamSerializerInfo
}

// StreamSerializerInfo contains information about a specific stream serialization format
type StreamSerializerInfo struct {
	// EncodesAsText indicates this serializer can be encoded to UTF-8 safely.
	EncodesAsText bool
	// Serializer is the top level object serializer for this type when streaming
	Serializer
	// Framer is the factory for retrieving streams that separate objects on the wire
	Framer
}

// NegotiatedSerializer is an interface used for obtaining encoders, decoders, and serializers
// for multiple supported media types. This would commonly be accepted by a server component
// that performs HTTP content negotiation to accept multiple formats.
type NegotiatedSerializer interface {
	// SupportedMediaTypes is the media types supported for reading and writing single objects.
	SupportedMediaTypes() []SerializerInfo

	// EncoderForVersion returns an encoder that ensures objects being written to the provided
	// serializer are in the provided group version.
	EncoderForVersion(serializer Encoder, gv GroupVersioner) Encoder
	// DecoderToVersion returns a decoder that ensures objects being read by the provided
	// serializer are in the provided group version by default.
	DecoderToVersion(serializer Decoder, gv GroupVersioner) Decoder
}

// ClientNegotiator handles turning an HTTP content type into the appropriate encoder.
// Use NewClientNegotiator or NewVersionedClientNegotiator to create this interface from
// a NegotiatedSerializer.
type ClientNegotiator interface {
	// Encoder returns the appropriate encoder for the provided contentType (e.g. application/json)
	// and any optional mediaType parameters (e.g. pretty=1), or an error. If no serializer is found
	// a NegotiateError will be returned. The current client implementations consider params to be
	// optional modifiers to the contentType and will ignore unrecognized parameters.
	Encoder(contentType string, params map[string]string) (Encoder, error)
	// Decoder returns the appropriate decoder for the provided contentType (e.g. application/json)
	// and any optional mediaType parameters (e.g. pretty=1), or an error. If no serializer is found
	// a NegotiateError will be returned. The current client implementations consider params to be
	// optional modifiers to the contentType and will ignore unrecognized parameters.
	Decoder(contentType string, params map[string]string) (Decoder, error)
	// StreamDecoder returns the appropriate stream decoder for the provided contentType (e.g.
	// application/json) and any optional mediaType parameters (e.g. pretty=1), or an error. If no
	// serializer is found a NegotiateError will be returned. The Serializer and Framer will always
	// be returned if a Decoder is returned. The current client implementations consider params to be
	// optional modifiers to the contentType and will ignore unrecognized parameters.
	StreamDecoder(contentType string, params map[string]string) (Decoder, Serializer, Framer, error)
}

// ParameterCodec defines methods for serializing and deserializing API objects to url.Values and
// performing any necessary conversion. Unlike the normal Codec, query parameters are not self describing
// and the desired version must be specified.
type ParameterCodec interface {
	// DecodeParameters takes the given url.Values in the specified group version and decodes them
	// into the provided object, or returns an error.
	DecodeParameters(parameters url.Values, from schema.GroupVersion, into Object) error
	// EncodeParameters encodes the provided object as query parameters or returns an error.
	EncodeParameters(obj Object, to schema.GroupVersion) (url.Values, error)
}

// Unstructured objects store values as map[string]interface{}, with only values that can be serialized
// to JSON allowed.
type Unstructured interface {
	Object
	// NewEmptyInstance returns a new instance of the concrete type containing only kind/apiVersion and no other data.
	// This should be called instead of reflect.New() for unstructured types because the go type alone does not preserve kind/apiVersion info.
	NewEmptyInstance() Unstructured
	// UnstructuredContent returns a non-nil map with this object's contents. Values may be
	// []interface{}, map[string]interface{}, or any primitive type. Contents are typically serialized to
	// and from JSON. SetUnstructuredContent should be used to mutate the contents.
	UnstructuredContent() map[string]interface{}
	// SetUnstructuredContent updates the object content to match the provided map.
	SetUnstructuredContent(map[string]interface{})
	// IsList returns true if this type is a list or matches the list convention - has an array called "items".
	IsList() bool
	// EachListItem should pass a single item out of the list as an Object to the provided function. Any
	// error should terminate the iteration. If IsList() returns false, this method should return an error
	// instead of calling the provided function.
	EachListItem(func(Object) error) error
}
