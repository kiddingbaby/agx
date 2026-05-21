package profile

import "fmt"

type NotFoundError struct {
	Name string
}

func (e *NotFoundError) Error() string {
	if e.Name == "" {
		return "relay not found"
	}
	return fmt.Sprintf("relay not found: %s", e.Name)
}
