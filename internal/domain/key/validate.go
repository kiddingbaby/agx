package key

import (
	"fmt"
	"strings"
)

func ValidateKeyName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("key name is required")
	}
	if strings.ContainsAny(name, "\x00\n\r") {
		return fmt.Errorf("key name contains invalid characters")
	}
	return nil
}

func ValidateAPIKey(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("api key is required")
	}
	if strings.ContainsAny(value, "\x00\n\r") {
		return fmt.Errorf("api key contains invalid characters")
	}
	return nil
}

func ValidateBaseURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\x00\n\r") {
		return fmt.Errorf("base-url contains invalid characters")
	}
	return nil
}
