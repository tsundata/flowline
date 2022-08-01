package uid

import "github.com/google/uuid"

func New() string {
	return uuid.NewString()
}

func IsValid(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}
