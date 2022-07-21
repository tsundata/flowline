package watch

import (
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/serializer/streaming"
	"github.com/tsundata/flowline/pkg/watch"
)

// Decoder implements the watch.Decoder interface for io.ReadClosers that
// have contents which consist of a series of watchEvent objects encoded
// with the given streaming decoder. The internal objects will be then
// decoded by the embedded decoder.
type Decoder struct {
	decoder         streaming.Decoder
	embeddedDecoder runtime.Decoder
}

// NewDecoder creates an Decoder for the given writer and codec.
func NewDecoder(decoder streaming.Decoder, embeddedDecoder runtime.Decoder) *Decoder {
	return &Decoder{
		decoder:         decoder,
		embeddedDecoder: embeddedDecoder,
	}
}

// Decode blocks until it can return the next object in the reader. Returns an error
// if the reader is closed or an object can't be decoded.
func (d *Decoder) Decode() (watch.EventType, runtime.Object, error) {
	var got meta.WatchEvent
	res, _, err := d.decoder.Decode(nil, &got)
	if err != nil {
		return "", nil, err
	}
	if res != &got {
		return "", nil, fmt.Errorf("unable to decode to meta.Event")
	}
	switch got.Type {
	case string(watch.Added), string(watch.Modified), string(watch.Deleted), string(watch.Error), string(watch.Bookmark):
	default:
		return "", nil, fmt.Errorf("got invalid watch event type: %v", got.Type)
	}

	obj, err := runtime.Decode(d.embeddedDecoder, got.Object.Raw, nil)
	if err != nil {
		return "", nil, fmt.Errorf("unable to decode watch event: %v", err)
	}
	return watch.EventType(got.Type), obj, nil
}

// Close closes the underlying r.
func (d *Decoder) Close() {
	d.decoder.Close()
}
