package key

import "testing"

func TestSupportedProviders(t *testing.T) {
	providers := SupportedProviders()
	if len(providers) != 3 {
		t.Fatalf("len(providers) = %d, want 3", len(providers))
	}

	providers[0] = "mutated"
	fresh := SupportedProviders()
	if fresh[0] != ProviderClaude {
		t.Fatalf("SupportedProviders should return a copy")
	}
}

func TestParseProvider(t *testing.T) {
	cases := []struct {
		input string
		want  Provider
		ok    bool
	}{
		{input: "claude", want: ProviderClaude, ok: true},
		{input: " OPENAI ", want: ProviderOpenAI, ok: true},
		{input: "gemini", want: ProviderGemini, ok: true},
		{input: "x", want: "", ok: false},
	}

	for _, tc := range cases {
		got, ok := ParseProvider(tc.input)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("ParseProvider(%q) = (%q,%v), want (%q,%v)", tc.input, got, ok, tc.want, tc.ok)
		}
	}
}

func TestProviderValid(t *testing.T) {
	if !ProviderClaude.Valid() {
		t.Fatal("ProviderClaude.Valid() = false, want true")
	}
	if Provider("invalid").Valid() {
		t.Fatal("Provider(\"invalid\").Valid() = true, want false")
	}
}

func TestParseRotationStrategy(t *testing.T) {
	cases := []struct {
		input string
		want  RotationStrategy
		ok    bool
	}{
		{input: "fixed", want: StrategyFixed, ok: true},
		{input: " ROUND_ROBIN ", want: StrategyRoundRobin, ok: true},
		{input: "random", want: StrategyRandom, ok: true},
		{input: "x", want: "", ok: false},
	}

	for _, tc := range cases {
		got, ok := ParseRotationStrategy(tc.input)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("ParseRotationStrategy(%q) = (%q,%v), want (%q,%v)", tc.input, got, ok, tc.want, tc.ok)
		}
	}
}

func TestNormalizeProfileName(t *testing.T) {
	if got := NormalizeProfileName(""); got != DefaultProfile {
		t.Fatalf("NormalizeProfileName(\"\") = %q, want %q", got, DefaultProfile)
	}
	if got := NormalizeProfileName(" PROD "); got != "prod" {
		t.Fatalf("NormalizeProfileName(\" PROD \") = %q, want %q", got, "prod")
	}
}
