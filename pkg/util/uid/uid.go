package uid

import "github.com/google/uuid"

func New() string {
	return uuid.NewString()
}
