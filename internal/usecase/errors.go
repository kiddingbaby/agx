package usecase

import (
	"errors"
	"fmt"
)

// UnknownAgentError indicates the given agent name is not registered.
type UnknownAgentError struct {
	Name string
}

func (e *UnknownAgentError) Error() string {
	return fmt.Sprintf("unknown agent: %s", e.Name)
}

// NoActiveKeyError indicates the provider has no active key.
type NoActiveKeyError struct {
	Provider string
}

func (e *NoActiveKeyError) Error() string {
	return fmt.Sprintf("no active key for %s", e.Provider)
}

// KeyNotFoundError indicates no key matches the input identifier.
type KeyNotFoundError struct {
	Identifier string
}

func (e *KeyNotFoundError) Error() string {
	if e.Identifier == "" {
		return "key not found"
	}
	return fmt.Sprintf("key not found: %s", e.Identifier)
}

func (e *KeyNotFoundError) Is(target error) bool {
	_, ok := target.(*KeyNotFoundError)
	return ok
}

// RuntimeError wraps failures from SessionRuntime operations.
type RuntimeError struct {
	Operation string
	Err       error
}

func (e *RuntimeError) Error() string {
	if e.Operation == "" {
		return fmt.Sprintf("runtime error: %v", e.Err)
	}
	return fmt.Sprintf("runtime %s failed: %v", e.Operation, e.Err)
}

func (e *RuntimeError) Unwrap() error {
	return e.Err
}

// ErrKeyNotFound is kept for compatibility with existing callers using errors.Is.
var ErrKeyNotFound error = &KeyNotFoundError{}

func IsUnknownAgentError(err error) bool {
	var target *UnknownAgentError
	return errors.As(err, &target)
}

func IsNoActiveKeyError(err error) bool {
	var target *NoActiveKeyError
	return errors.As(err, &target)
}

func IsKeyNotFoundError(err error) bool {
	var target *KeyNotFoundError
	return errors.As(err, &target)
}

func IsRuntimeError(err error) bool {
	var target *RuntimeError
	return errors.As(err, &target)
}

func WrapRuntimeError(operation string, err error) error {
	if err == nil {
		return nil
	}

	var runtimeErr *RuntimeError
	if errors.As(err, &runtimeErr) {
		return err
	}

	return &RuntimeError{
		Operation: operation,
		Err:       err,
	}
}
