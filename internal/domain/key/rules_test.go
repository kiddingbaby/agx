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
