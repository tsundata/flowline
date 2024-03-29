package util

import (
	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/tsundata/flowline/pkg/api/meta"
)

// ValidateEventType checks that eventtype is an expected type of event
func ValidateEventType(eventtype string) bool {
	switch eventtype {
	case meta.EventTypeNormal, meta.EventTypeWarning:
		return true
	}
	return false
}

// IsKeyNotFoundError is utility function that checks if an error is not found error
func IsKeyNotFoundError(err error) bool {
	//statusErr, _ := err.(*errors.StatusError)

	//return statusErr != nil && statusErr.Status().Code == http.StatusNotFound
	return false
}

func CreateTwoWayMergePatch(oldData []byte, newData []byte, dataStruct interface{}) ([]byte, error) {
	return jsonpatch.CreateMergePatch(oldData, newData)
}
