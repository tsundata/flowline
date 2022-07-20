package runtime

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type simpleNegotiatedSerializer struct {
	info SerializerInfo
}

func NewSimpleNegotiatedSerializer(info SerializerInfo) NegotiatedSerializer {
	return &simpleNegotiatedSerializer{info: info}
}

func (n *simpleNegotiatedSerializer) SupportedMediaTypes() []SerializerInfo {
	return []SerializerInfo{n.info}
}

func (n *simpleNegotiatedSerializer) EncoderForVersion(e Encoder, _ GroupVersioner) Encoder {
	return e
}

func (n *simpleNegotiatedSerializer) DecoderToVersion(d Decoder, _gv GroupVersioner) Decoder {
	return d
}

// NegotiateError is returned when a ClientNegotiator is unable to locate
// a serializer for the requested operation.
type NegotiateError struct {
	ContentType string
	Stream      bool
}

func (e NegotiateError) Error() string {
	if e.Stream {
		return fmt.Sprintf("no stream serializers registered for %s", e.ContentType)
	}
	return fmt.Sprintf("no serializers registered for %s", e.ContentType)
}

type clientNegotiator struct {
	serializer     NegotiatedSerializer
	encode, decode GroupVersioner
}

func (n *clientNegotiator) Encoder(contentType string, params map[string]string) (Encoder, error) {
	// if client negotiators truly need to use it
	mediaTypes := n.serializer.SupportedMediaTypes()
	info, ok := SerializerInfoForMediaType(mediaTypes, contentType)
	if !ok {
		if len(contentType) != 0 || len(mediaTypes) == 0 {
			return nil, NegotiateError{ContentType: contentType}
		}
		info = mediaTypes[0]
	}
	return n.serializer.EncoderForVersion(info.Serializer, n.encode), nil
}

func (n *clientNegotiator) Decoder(contentType string, params map[string]string) (Decoder, error) {
	mediaTypes := n.serializer.SupportedMediaTypes()
	info, ok := SerializerInfoForMediaType(mediaTypes, contentType)
	if !ok {
		if len(contentType) != 0 || len(mediaTypes) == 0 {
			return nil, NegotiateError{ContentType: contentType}
		}
		info = mediaTypes[0]
	}
	return n.serializer.DecoderToVersion(info.Serializer, n.decode), nil
}

func (n *clientNegotiator) StreamDecoder(contentType string, params map[string]string) (Decoder, Serializer, Framer, error) {
	mediaTypes := n.serializer.SupportedMediaTypes()
	info, ok := SerializerInfoForMediaType(mediaTypes, contentType)
	if !ok {
		if len(contentType) != 0 || len(mediaTypes) == 0 {
			return nil, nil, nil, NegotiateError{ContentType: contentType, Stream: true}
		}
		info = mediaTypes[0]
	}
	if info.StreamSerializer == nil {
		return nil, nil, nil, NegotiateError{ContentType: info.MediaType, Stream: true}
	}
	return n.serializer.DecoderToVersion(info.Serializer, n.decode), info.StreamSerializer.Serializer, info.StreamSerializer.Framer, nil
}

// NewClientNegotiator will attempt to retrieve the appropriate encoder, decoder, or
// stream decoder for a given content type. Does not perform any conversion, but will
// encode the object to the desired group, version, and kind. Use when creating a client.
func NewClientNegotiator(serializer NegotiatedSerializer, gv schema.GroupVersion) ClientNegotiator {
	return &clientNegotiator{
		serializer: serializer,
		encode:     gv,
	}
}
