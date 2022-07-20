package runtime

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"github.com/tsundata/flowline/pkg/runtime/schema"
	"github.com/tsundata/flowline/pkg/util/flog"
	"io"
)

// Serializer is the core interface for transforming objects into a serialized format and back.
// Implementations may choose to perform conversion of the object, but no assumptions should be made.
type Serializer interface {
	Encoder
	Decoder
}

// Codec is a Serializer that deals with the details of versioning objects. It offers the same
// interface as Serializer, so this is a marker to consumers that care about the version of the objects
// they receive.
type Codec Serializer

// Identifier represents an identifier.
// Identitier of two different objects should be equal if and only if for every
// input the output they produce is exactly the same.
type Identifier string

// Encoder writes objects to a serialized form
type Encoder interface {
	// Encode writes an object to a stream. Implementations may return errors if the versions are
	// incompatible, or if no conversion is defined.
	Encode(obj Object, w io.Writer) error
	// Identifier returns an identifier of the encoder.
	// Identifiers of two different encoders should be equal if and only if for every input
	// object it will be encoded to the same representation by both of them.
	//
	// Identifier is intended for use with CacheableObject#CacheEncode method. In order to
	// correctly handle CacheableObject, Encode() method should look similar to below, where
	// doEncode() is the encoding logic of implemented encoder:
	//   func (e *MyEncoder) Encode(obj Object, w io.Writer) error {
	//     if co, ok := obj.(CacheableObject); ok {
	//       return co.CacheEncode(e.Identifier(), e.doEncode, w)
	//     }
	//     return e.doEncode(obj, w)
	//   }
	Identifier() Identifier
}

// Decoder attempts to load an object from data.
type Decoder interface {
	// Decode attempts to deserialize the provided data using either the innate typing of the scheme or the
	// default kind, group, and version provided. It returns a decoded object as well as the kind, group, and
	// version from the serialized data, or an error. If into is non-nil, it will be used as the target type
	// and implementations may choose to use it rather than reallocating an object. However, the object is not
	// guaranteed to be populated. The returned object is not guaranteed to match into. If defaults are
	// provided, they are applied to the data by default. If no defaults or partial defaults are provided, the
	// type of the into may be used to guide conversion decisions.
	Decode(data []byte, defaults *schema.GroupVersionKind, into Object) (Object, *schema.GroupVersionKind, error)
}

type base64Serializer struct {
	Encoder
	Decoder

	identifier Identifier
}

func NewBase64Serializer(e Encoder, d Decoder) Serializer {
	return &base64Serializer{
		Encoder:    e,
		Decoder:    d,
		identifier: identifier(e),
	}
}

func identifier(e Encoder) Identifier {
	result := map[string]string{
		"name": "base64",
	}
	if e != nil {
		result["encoder"] = string(e.Identifier())
	}
	identifier, err := json.Marshal(result)
	if err != nil {
		flog.Fatalf("Failed marshaling identifier for base64Serializer: %v", err)
	}
	return Identifier(identifier)
}

func (s base64Serializer) Encode(obj Object, stream io.Writer) error {
	return s.doEncode(obj, stream)
}

func (s base64Serializer) doEncode(obj Object, stream io.Writer) error {
	e := base64.NewEncoder(base64.StdEncoding, stream)
	err := s.Encoder.Encode(obj, e)
	_ = e.Close()
	return err
}

// Identifier implements runtime.Encoder interface.
func (s base64Serializer) Identifier() Identifier {
	return s.identifier
}

func (s base64Serializer) Decode(data []byte, defaults *schema.GroupVersionKind, into Object) (Object, *schema.GroupVersionKind, error) {
	out := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
	n, err := base64.StdEncoding.Decode(out, data)
	if err != nil {
		return nil, nil, err
	}
	return s.Decoder.Decode(out[:n], defaults, into)
}

// Encode is a convenience wrapper for encoding to a []byte from an Encoder
func Encode(e Encoder, obj Object) ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := e.Encode(obj, buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode is a convenience wrapper for decoding data into an Object.
func Decode(d Decoder, data []byte, obj Object) (Object, error) {
	obj, _, err := d.Decode(data, nil, obj)
	return obj, err
}

type JsonCoder struct{}

func (c JsonCoder) Decode(data []byte, _ *schema.GroupVersionKind, into Object) (Object, *schema.GroupVersionKind, error) {
	err := json.Unmarshal(data, &into)
	if err != nil {
		return nil, nil, err
	}
	return into, nil, nil
}

func (c JsonCoder) Encode(obj Object, w io.Writer) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func (c JsonCoder) Identifier() Identifier {
	return "json"
}

// SerializerInfoForMediaType returns the first info in types that has a matching media type (which cannot
// include media-type parameters), or the first info with an empty media type, or false if no type matches.
func SerializerInfoForMediaType(types []SerializerInfo, mediaType string) (SerializerInfo, bool) {
	for _, info := range types {
		if info.MediaType == mediaType {
			return info, true
		}
	}
	for _, info := range types {
		if len(info.MediaType) == 0 {
			return info, true
		}
	}
	return SerializerInfo{}, false
}
