package domain

import (
	"errors"
	"fmt"
)

// Sentinel errors the transport layer maps onto HTTP status codes.
var (
	// ErrNotFound is returned when an entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrUnavailable is returned when live data has never been fetched.
	ErrUnavailable = errors.New("device data unavailable")
	// ErrUpstreamAuth is returned when the provider rejected our credentials.
	ErrUpstreamAuth = errors.New("upstream authentication failed")
)

// ValidationError describes a rejected field so the UI can highlight it.
type ValidationError struct {
	Field   string
	Message string
}

// NewValidationError builds a ValidationError.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// AsValidationError reports whether err is (or wraps) a ValidationError.
func AsValidationError(err error) (*ValidationError, bool) {
	var v *ValidationError
	if errors.As(err, &v) {
		return v, true
	}
	return nil, false
}
