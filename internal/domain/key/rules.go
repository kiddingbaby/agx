package key

import "strings"

var supportedProviders = []Provider{
	ProviderClaude,
	ProviderOpenAI,
	ProviderGemini,
}

var supportedStrategies = []RotationStrategy{
	StrategyFixed,
	StrategyRoundRobin,
	StrategyRandom,
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

// SupportedStrategies returns a copy of allowed key rotation strategies.
func SupportedStrategies() []RotationStrategy {
	out := make([]RotationStrategy, len(supportedStrategies))
	copy(out, supportedStrategies)
	return out
}

// ParseRotationStrategy parses a strategy and checks if it is supported.
func ParseRotationStrategy(raw string) (RotationStrategy, bool) {
	candidate := RotationStrategy(strings.TrimSpace(strings.ToLower(raw)))
	for _, s := range supportedStrategies {
		if s == candidate {
			return s, true
		}
	}
	return "", false
}

// Valid reports whether the strategy is supported.
func (s RotationStrategy) Valid() bool {
	_, ok := ParseRotationStrategy(string(s))
	return ok
}

// NormalizeProfileName normalizes profile name and falls back to default.
func NormalizeProfileName(raw string) string {
	name := strings.TrimSpace(strings.ToLower(raw))
	if name == "" {
		return DefaultProfile
	}
	return name
}
