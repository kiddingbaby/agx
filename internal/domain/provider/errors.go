package provider

import (
	"errors"
	"fmt"
)

// TargetNotFoundError indicates a provider target does not exist.
type TargetNotFoundError struct {
	Name string
}

func (e *TargetNotFoundError) Error() string {
	name := e.Name
	if name == "" {
		return "target not found"
	}
	return fmt.Sprintf("target not found: %s", name)
}

func (e *TargetNotFoundError) Is(target error) bool {
	_, ok := target.(*TargetNotFoundError)
	return ok
}

func IsTargetNotFoundError(err error) bool {
	var target *TargetNotFoundError
	return errors.As(err, &target)
}
