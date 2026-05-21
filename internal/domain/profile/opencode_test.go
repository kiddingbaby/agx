package profile

import "testing"

func TestOpenCodeProviderIDFor(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		family  OpenCodeProviderFamily
		want    string
	}{
		{"openai", "newapi", OpenCodeProviderFamilyOpenAICompatible, "agx-newapi-openai-compatible"},
		{"anthropic", "newapi", OpenCodeProviderFamilyAnthropic, "agx-newapi-anthropic"},
		{"gemini", "newapi", OpenCodeProviderFamilyGemini, "agx-newapi-gemini"},
		{"empty name", "", OpenCodeProviderFamilyOpenAICompatible, ""},
		{"empty family", "newapi", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OpenCodeProviderIDFor(tt.profile, tt.family); got != tt.want {
				t.Fatalf("OpenCodeProviderIDFor(%q,%q) = %q, want %q", tt.profile, tt.family, got, tt.want)
			}
		})
	}
}

func TestOpenCodeDefaultFamilyForModel(t *testing.T) {
	tests := []struct {
		name    string
		modelID string
		want    OpenCodeProviderFamily
	}{
		{"claude full", "claude-sonnet-4-5", OpenCodeProviderFamilyAnthropic},
		{"opus short", "opus-4.7", OpenCodeProviderFamilyAnthropic},
		{"sonnet short", "sonnet-4-5", OpenCodeProviderFamilyAnthropic},
		{"haiku short", "haiku-3-5", OpenCodeProviderFamilyAnthropic},
		{"gemini", "gemini-2.5-pro", OpenCodeProviderFamilyGemini},
		{"gpt", "gpt-4o", OpenCodeProviderFamilyOpenAICompatible},
		{"deepseek", "deepseek-chat", OpenCodeProviderFamilyOpenAICompatible},
		{"empty", "", OpenCodeProviderFamilyOpenAICompatible},
		{"mixed case claude", "Claude-3-Opus", OpenCodeProviderFamilyAnthropic},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OpenCodeDefaultFamilyForModel(tt.modelID); got != tt.want {
				t.Fatalf("OpenCodeDefaultFamilyForModel(%q) = %q, want %q", tt.modelID, got, tt.want)
			}
		})
	}
}

func TestOpenCodeManagedFamiliesStable(t *testing.T) {
	families := OpenCodeManagedFamilies()
	if len(families) != 3 {
		t.Fatalf("families = %v, want 3 entries", families)
	}
	// Mutating the returned slice must not affect future callers.
	families[0] = OpenCodeProviderFamily("mutated")
	if got := OpenCodeManagedFamilies()[0]; got != OpenCodeProviderFamilyOpenAICompatible {
		t.Fatalf("OpenCodeManagedFamilies should return a fresh slice; got %q", got)
	}
}
