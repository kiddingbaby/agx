package profile

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

func ValidateProfileName(name string) error {
	name = NormalizeProfileName(name)
	if name == "" {
		return fmt.Errorf("relay name is required")
	}
	for _, r := range name {
		if unicode.IsSpace(r) {
			return fmt.Errorf("relay name cannot contain whitespace")
		}
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.') {
			return fmt.Errorf("relay name contains invalid characters")
		}
	}
	return nil
}

func NormalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.Path != "/" {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
	}
	parsed.Fragment = ""
	return parsed.String()
}

func ValidateBaseURL(raw string) error {
	value := NormalizeBaseURL(raw)
	if value == "" {
		return fmt.Errorf("base url is required")
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("base url is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("base url must start with http:// or https://")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("base url host is required")
	}
	if strings.ContainsAny(value, "\x00\n\r") {
		return fmt.Errorf("base url contains invalid characters")
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
