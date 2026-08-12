package domain

import (
	"errors"
	"fmt"
)


var (

	ErrNotFound = errors.New("not found")

	ErrUnavailable = errors.New("device data unavailable")
	
	ErrUpstreamAuth = errors.New("upstream authentication failed")
)


type ValidationError struct {
	Field   string
	Message string
}


func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}


func AsValidationError(err error) (*ValidationError, bool) {
	var v *ValidationError
	if errors.As(err, &v) {
		return v, true
	}
	return nil, false
}
