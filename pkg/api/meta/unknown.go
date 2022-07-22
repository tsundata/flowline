package meta

import (
	"github.com/tsundata/flowline/pkg/runtime"
	"github.com/tsundata/flowline/pkg/runtime/schema"
)

type RawExtension struct {
	// Raw is the underlying serialization of this object.
	Raw []byte `json:"raw,omitempty"`
	// Object can hold a representation of this extension - useful for working with versioned
	// structs.
	Object runtime.Object `json:"object,omitempty"`
}

func (m *RawExtension) GetObjectKind() schema.ObjectKind {
	//TODO implement me
	panic("implement me")
}

func (m *RawExtension) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}

type Unknown struct {
	TypeMeta `json:",inline"`
	// Raw will hold the complete serialized object which couldn't be matched
	// with a registered type. Most likely, nothing should be done with this
	// except for passing it through the system.
	Raw []byte
	// ContentEncoding is encoding used to encode 'Raw' data.
	// Unspecified means no encoding.
	ContentEncoding string
	// ContentType  is serialization method used to serialize 'Raw'.
	// Unspecified means ContentTypeJSON.
	ContentType string
}

func (m *Unknown) GetObjectKind() schema.ObjectKind {
	return m
}

func (m *Unknown) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}
