package usecase

import (
	"errors"
	"fmt"
)

// NoActiveKeyError indicates the provider has no active key.
type NoActiveKeyError struct {
	Provider string
	Profile  string
}

func (e *NoActiveKeyError) Error() string {
	if e.Profile != "" {
		return fmt.Sprintf("no active key for %s (profile=%s)", e.Provider, e.Profile)
	}
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

// KeySelectorNoMatchError indicates no key matches the requested tag selector in a given scope.
type KeySelectorNoMatchError struct {
	Provider string
	Profile  string
	Tags     []string
}

func (e *KeySelectorNoMatchError) Error() string {
	scope := e.Provider
	if e.Profile != "" {
		scope = fmt.Sprintf("%s (profile=%s)", e.Provider, e.Profile)
	}
	return fmt.Sprintf("no key matches selector for %s: tags=%v", scope, e.Tags)
}

// KeySelectorAmbiguousError indicates multiple keys match a selector without a single active choice.
type KeySelectorAmbiguousError struct {
	Provider string
	Profile  string
	Tags     []string
	Matches  []string
}

func (e *KeySelectorAmbiguousError) Error() string {
	scope := e.Provider
	if e.Profile != "" {
		scope = fmt.Sprintf("%s (profile=%s)", e.Provider, e.Profile)
	}
	if len(e.Matches) == 0 {
		return fmt.Sprintf("multiple keys match selector for %s: tags=%v", scope, e.Tags)
	}
	return fmt.Sprintf("multiple keys match selector for %s: tags=%v matches=%v", scope, e.Tags, e.Matches)
}

func (e *KeyNotFoundError) Is(target error) bool {
	_, ok := target.(*KeyNotFoundError)
	return ok
}

// ErrKeyNotFound is kept for compatibility with existing callers using errors.Is.
var ErrKeyNotFound error = &KeyNotFoundError{}

func IsNoActiveKeyError(err error) bool {
	var target *NoActiveKeyError
	return errors.As(err, &target)
}

func IsKeyNotFoundError(err error) bool {
	var target *KeyNotFoundError
	return errors.As(err, &target)
}

func IsKeySelectorNoMatchError(err error) bool {
	var target *KeySelectorNoMatchError
	return errors.As(err, &target)
}

func IsKeySelectorAmbiguousError(err error) bool {
	var target *KeySelectorAmbiguousError
	return errors.As(err, &target)
}
