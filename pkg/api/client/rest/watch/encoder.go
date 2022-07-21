package watch

import (
	"encoding/json"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/serializer/streaming"
	"github.com/tsundata/flowline/pkg/watch"
)

// Encoder serializes watch.Events into io.Writer. The internal objects
// are encoded using embedded encoder, and the outer Event is serialized
// using encoder.
// TODO: this type is only used by tests
type Encoder struct {
	encoder         streaming.Encoder
	embeddedEncoder runtime.Encoder
}

func NewEncoder(encoder streaming.Encoder, embeddedEncoder runtime.Encoder) *Encoder {
	return &Encoder{
		encoder:         encoder,
		embeddedEncoder: embeddedEncoder,
	}
}

// Encode writes an event to the writer. Returns an error
// if the writer is closed or an object can't be encoded.
func (e *Encoder) Encode(event *watch.Event) error {
	data, err := runtime.Encode(e.embeddedEncoder, event.Object)
	if err != nil {
		return err
	}
	return e.encoder.Encode(&meta.WatchEvent{
		Type:   string(event.Type),
		Object: meta.RawExtension{Raw: json.RawMessage(data)},
	})
}
