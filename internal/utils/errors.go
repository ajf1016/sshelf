package utils

import "fmt"

// ErrNotFound is returned when a named resource does not exist.
type ErrNotFound struct {
	Resource string
	Name     string
}

func (e *ErrNotFound) Error() string {
	if e.Name == "" {
		return fmt.Sprintf("no %s", e.Resource)
	}
	return fmt.Sprintf("%s %q not found", e.Resource, e.Name)
}

// ErrAlreadyExists is returned when creating a resource whose name is already taken.
type ErrAlreadyExists struct {
	Resource string
	Name     string
}

func (e *ErrAlreadyExists) Error() string {
	return fmt.Sprintf("%s %q already exists", e.Resource, e.Name)
}

// ErrInvalidInput is returned when a field fails validation.
type ErrInvalidInput struct {
	Field   string
	Message string
}

func (e *ErrInvalidInput) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
}
