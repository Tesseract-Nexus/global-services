package services

import (
	"errors"
	"fmt"
)

// ValidationError represents a validation failure with suggestions
type ValidationError struct {
	Field       string   `json:"field"`
	Message     string   `json:"message"`
	Suggestions []string `json:"suggestions,omitempty"`
}

func (e *ValidationError) Error() string {
	if len(e.Suggestions) > 0 {
		return fmt.Sprintf("%s: %s. Suggestions: %v", e.Field, e.Message, e.Suggestions)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string, suggestions []string) *ValidationError {
	return &ValidationError{
		Field:       field,
		Message:     message,
		Suggestions: suggestions,
	}
}

// IsValidationError checks if an error is a ValidationError
func IsValidationError(err error) (*ValidationError, bool) {
	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		return validationErr, true
	}
	return nil, false
}

// ConflictError represents a resource conflict (e.g., already exists)
type ConflictError struct {
	Resource string `json:"resource"`
	Message  string `json:"message"`
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s conflict: %s", e.Resource, e.Message)
}

// NewConflictError creates a new conflict error
func NewConflictError(resource, message string) *ConflictError {
	return &ConflictError{
		Resource: resource,
		Message:  message,
	}
}

// IsConflictError checks if an error is a ConflictError
func IsConflictError(err error) (*ConflictError, bool) {
	var conflictErr *ConflictError
	if errors.As(err, &conflictErr) {
		return conflictErr, true
	}
	return nil, false
}
