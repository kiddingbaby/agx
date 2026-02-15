package key

import "strings"

var supportedProviders = []Provider{
	ProviderClaude,
	ProviderOpenAI,
	ProviderGemini,
}

// SupportedProviders returns a copy of allowed providers.
func SupportedProviders() []Provider {
	out := make([]Provider, len(supportedProviders))
	copy(out, supportedProviders)
	return out
}

// ParseProvider parses a provider name and checks if it is supported.
func ParseProvider(raw string) (Provider, bool) {
	candidate := Provider(strings.TrimSpace(strings.ToLower(raw)))
	for _, p := range supportedProviders {
		if p == candidate {
			return p, true
		}
	}
	return "", false
}

// Valid reports whether the provider is supported.
func (p Provider) Valid() bool {
	_, ok := ParseProvider(string(p))
	return ok
}
