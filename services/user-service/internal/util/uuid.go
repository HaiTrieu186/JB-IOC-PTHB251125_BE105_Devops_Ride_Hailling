package util

import (
	"github.com/google/uuid"
)

func IsValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func ParseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
