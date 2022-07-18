package value

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Context is additional information that a storage transformation may need to verify the data at rest.
type Context interface {
	// AuthenticatedData should return an array of bytes that describes the current value. If the value changes,
	// the transformer may report the value as unreadable or tampered. This may be nil if no such description exists
	// or is needed. For additional verification, set this to data that strongly identifies the value, such as
	// the key and creation version of the stored data.
	AuthenticatedData() []byte
}

// Transformer allows a value to be transformed before being read from or written to the underlying store. The methods
// must be able to undo the transformation caused by the other.
type Transformer interface {
	// TransformFromStorage may transform the provided data from its underlying storage representation or return an error.
	// Stale is true if the object on disk is stale and a write to etcd should be issued, even if the contents of the object
	// have not changed.
	TransformFromStorage(ctx context.Context, data []byte, dataCtx Context) (out []byte, stale bool, err error)
	// TransformToStorage may transform the provided data into the appropriate form in storage or return an error.
	TransformToStorage(ctx context.Context, data []byte, dataCtx Context) (out []byte, err error)
}

type identityTransformer struct{}

// IdentityTransformer performs no transformation of the provided data.
var IdentityTransformer Transformer = identityTransformer{}

func (identityTransformer) TransformFromStorage(ctx context.Context, data []byte, dataCtx Context) ([]byte, bool, error) {
	return data, false, nil
}
func (identityTransformer) TransformToStorage(ctx context.Context, data []byte, dataCtx Context) ([]byte, error) {
	return data, nil
}

// DefaultContext is a simple implementation of Context for a slice of bytes.
type DefaultContext []byte

// AuthenticatedData returns itself.
func (c DefaultContext) AuthenticatedData() []byte { return []byte(c) }

// MutableTransformer allows a transformer to be changed safely at runtime.
type MutableTransformer struct {
	lock        sync.RWMutex
	transformer Transformer
}

// NewMutableTransformer creates a transformer that can be updated at any time by calling Set()
func NewMutableTransformer(transformer Transformer) *MutableTransformer {
	return &MutableTransformer{transformer: transformer}
}

// Set updates the nested transformer.
func (t *MutableTransformer) Set(transformer Transformer) {
	t.lock.Lock()
	t.transformer = transformer
	t.lock.Unlock()
}

func (t *MutableTransformer) TransformFromStorage(ctx context.Context, data []byte, dataCtx Context) (out []byte, stale bool, err error) {
	t.lock.RLock()
	transformer := t.transformer
	t.lock.RUnlock()
	return transformer.TransformFromStorage(ctx, data, dataCtx)
}
func (t *MutableTransformer) TransformToStorage(ctx context.Context, data []byte, dataCtx Context) (out []byte, err error) {
	t.lock.RLock()
	transformer := t.transformer
	t.lock.RUnlock()
	return transformer.TransformToStorage(ctx, data, dataCtx)
}

// PrefixTransformer holds a transformer interface and the prefix that the transformation is located under.
type PrefixTransformer struct {
	Prefix      []byte
	Transformer Transformer
}

type prefixTransformers struct {
	transformers []PrefixTransformer
	err          error
}

var _ Transformer = &prefixTransformers{}

// NewPrefixTransformers supports the Transformer interface by checking the incoming data against the provided
// prefixes in order. The first matching prefix will be used to transform the value (the prefix is stripped
// before the Transformer interface is invoked). The first provided transformer will be used when writing to
// the store.
func NewPrefixTransformers(err error, transformers ...PrefixTransformer) Transformer {
	if err == nil {
		err = fmt.Errorf("the provided value does not match any of the supported transformers")
	}
	return &prefixTransformers{
		transformers: transformers,
		err:          err,
	}
}

// TransformFromStorage finds the first transformer with a prefix matching the provided data and returns
// the result of transforming the value. It will always mark any transformation as stale that is not using
// the first transformer.
func (t *prefixTransformers) TransformFromStorage(ctx context.Context, data []byte, dataCtx Context) ([]byte, bool, error) {
	var errs []error
	for i, transformer := range t.transformers {
		if bytes.HasPrefix(data, transformer.Prefix) {
			result, stale, err := transformer.Transformer.TransformFromStorage(ctx, data[len(transformer.Prefix):], dataCtx)
			// To migrate away from encryption, user can specify an identity transformer higher up
			// (in the config file) than the encryption transformer. In that scenario, the identity transformer needs to
			// identify (during reads from disk) whether the data being read is encrypted or not. If the data is encrypted,
			// it shall throw an error, but that error should not prevent the next subsequent transformer from being tried.
			if len(transformer.Prefix) == 0 && err != nil {
				continue
			}

			// It is valid to have overlapping prefixes when the same encryption provider
			// is specified multiple times but with different keys (the first provider is
			// being rotated to and some later provider is being rotated away from).
			//
			// Example:
			//
			//  {
			//    "aescbc": {
			//      "keys": [
			//        {
			//          "name": "2",
			//          "secret": "some key 2"
			//        }
			//      ]
			//    }
			//  },
			//  {
			//    "aescbc": {
			//      "keys": [
			//        {
			//          "name": "1",
			//          "secret": "some key 1"
			//        }
			//      ]
			//    }
			//  },
			//
			// The transformers for both aescbc configs share the prefix k8s:enc:aescbc:v1:
			// but a failure in the first one should not prevent a later match from being attempted.
			// Thus we never short-circuit on a prefix match that results in an error.
			if err != nil {
				errs = append(errs, err)
				continue
			}

			return result, stale || i != 0, err
		}
	}
	if errs != nil {
		errStr := strings.Builder{}
		for _, err := range errs {
			errStr.WriteString(err.Error())
			errStr.WriteString("  ")
		}
		return nil, false, errors.New(errStr.String())
	}

	return nil, false, t.err
}

// TransformToStorage uses the first transformer and adds its prefix to the data.
func (t *prefixTransformers) TransformToStorage(ctx context.Context, data []byte, dataCtx Context) ([]byte, error) {
	transformer := t.transformers[0]
	prefixedData := make([]byte, len(transformer.Prefix), len(data)+len(transformer.Prefix))
	copy(prefixedData, transformer.Prefix)
	result, err := transformer.Transformer.TransformToStorage(ctx, data, dataCtx)
	if err != nil {
		return nil, err
	}
	prefixedData = append(prefixedData, result...)
	return prefixedData, nil
}