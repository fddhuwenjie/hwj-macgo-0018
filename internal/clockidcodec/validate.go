package clockidcodec

import (
	"errors"
	"strings"
)

var ErrInvalidID = errors.New("clockidcodec: invalid id")

func ValidateID(id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidID
	}
	if len(id) > 240 {
		return ErrInvalidID
	}
	for _, r := range id {
		if !(r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return ErrInvalidID
		}
	}
	return nil
}

func ValidatePrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	if len(prefix) > 80 {
		return ErrInvalidID
	}
	for _, r := range prefix {
		if !(r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return ErrInvalidID
		}
	}
	return nil
}
