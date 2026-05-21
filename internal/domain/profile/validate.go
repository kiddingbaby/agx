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
	if err := validateNameRunes(name, "relay name"); err != nil {
		return err
	}
	return nil
}

func isAllowedNameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.'
}

// validateNameRunes enforces the character set for profile and target names
// and additionally rejects names that resolve to filesystem traversal tokens
// (".", "..", "..." etc.) once interpolated into managed-context paths.
func validateNameRunes(name, kind string) error {
	hasAlnum := false
	for _, r := range name {
		if unicode.IsSpace(r) {
			return fmt.Errorf("%s cannot contain whitespace", kind)
		}
		if r == '/' || r == '\\' {
			return fmt.Errorf("%s contains invalid characters", kind)
		}
		if !isAllowedNameRune(r) {
			return fmt.Errorf("%s contains invalid characters", kind)
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			hasAlnum = true
		}
	}
	if !hasAlnum {
		return fmt.Errorf("%s must contain at least one letter or digit", kind)
	}
	return nil
}

func (k ProfileKind) Normalized() ProfileKind {
	return ProfileKind(strings.TrimSpace(strings.ToLower(string(k))))
}

func (k ProfileKind) Valid() bool {
	return k.Normalized() == ProfileKindRelay
}

func NormalizeProfile(raw Profile) Profile {
	raw.Name = NormalizeProfileName(raw.Name)
	raw.Kind = raw.Kind.Normalized()
	raw.BaseURL = NormalizeBaseURL(raw.BaseURL)
	raw.APIKey = strings.TrimSpace(raw.APIKey)
	raw.ModelID = strings.TrimSpace(raw.ModelID)
	raw.ProviderFamily = OpenCodeProviderFamily(strings.TrimSpace(strings.ToLower(string(raw.ProviderFamily))))
	if raw.Kind == "" {
		raw.Kind = ProfileKindRelay
	}
	return raw
}

func ValidateProfile(raw Profile) error {
	profile := NormalizeProfile(raw)
	if err := ValidateProfileName(profile.Name); err != nil {
		return err
	}
	if !profile.Kind.Valid() {
		return fmt.Errorf("profile kind must be %s", ProfileKindRelay)
	}
	if err := ValidateBaseURL(profile.BaseURL); err != nil {
		return err
	}
	if profile.APIKey == "" {
		return fmt.Errorf("relay credential is required")
	}
	if err := ValidateAPIKey(profile.APIKey); err != nil {
		return err
	}
	if profile.ProviderFamily != "" && !profile.ProviderFamily.Valid() {
		return fmt.Errorf("provider family must be one of: %s, %s, %s", OpenCodeProviderFamilyOpenAICompatible, OpenCodeProviderFamilyAnthropic, OpenCodeProviderFamilyGemini)
	}
	if strings.ContainsAny(profile.ModelID, "\x00\n\r") {
		return fmt.Errorf("model contains invalid characters")
	}
	return nil
}

func ResolveCredential(profile Profile) (string, error) {
	profile = NormalizeProfile(profile)
	if profile.APIKey == "" {
		return "", fmt.Errorf("relay credential is required")
	}
	return profile.APIKey, nil
}

func ValidateTargetName(name string) error {
	name = NormalizeTargetName(name)
	if name == "" {
		return fmt.Errorf("target name is required")
	}
	return validateNameRunes(name, "target name")
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
