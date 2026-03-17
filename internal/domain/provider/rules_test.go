package provider

import "testing"

func TestSupportedFamiliesReturnsCopy(t *testing.T) {
	families := SupportedFamilies()
	if len(families) != 3 {
		t.Fatalf("len(families) = %d, want 3", len(families))
	}
	families[0] = "mutated"
	fresh := SupportedFamilies()
	if fresh[0] != FamilyClaude {
		t.Fatalf("SupportedFamilies should return a copy")
	}
}

func TestParseFamily(t *testing.T) {
	got, ok := ParseFamily(" OPENAI ")
	if !ok || got != FamilyOpenAI {
		t.Fatalf("ParseFamily() = (%q,%v), want (%q,true)", got, ok, FamilyOpenAI)
	}
}

func TestCompatibleKind(t *testing.T) {
	if !CompatibleKind(FamilyOpenAI, KindOpenAICompatible) {
		t.Fatal("openai family should accept openai-compatible")
	}
	if CompatibleKind(FamilyClaude, KindOpenAICompatible) {
		t.Fatal("claude family should reject openai-compatible")
	}
}

func TestValidateTarget(t *testing.T) {
	valid := Target{
		Name:    "openrouter",
		Family:  FamilyOpenAI,
		Kind:    KindOpenAICompatible,
		Access:  AccessThirdParty,
		Auth:    AuthAPIKey,
		BaseURL: "https://openrouter.ai/api/v1",
	}
	if err := ValidateTarget(valid); err != nil {
		t.Fatalf("ValidateTarget(valid) error = %v", err)
	}

	tests := []Target{
		{
			Name:    "bad-wire-api",
			Family:  FamilyOpenAI,
			Kind:    KindOpenAICompatible,
			Access:  AccessThirdParty,
			Auth:    AuthAPIKey,
			BaseURL: "https://openrouter.ai/api/v1",
			WireAPI: WireAPI("nope"),
		},
		{
			Name:    "wire-api-on-claude",
			Family:  FamilyClaude,
			Kind:    KindClaude,
			Access:  AccessThirdParty,
			Auth:    AuthAPIKey,
			BaseURL: "https://claude-proxy.local",
			WireAPI: WireAPIResponses,
		},
		{
			Name:    "requires-auth-on-gemini",
			Family:  FamilyGemini,
			Kind:    KindGemini,
			Access:  AccessThirdParty,
			Auth:    AuthAPIKey,
			BaseURL: "https://gemini-proxy.local",
			RequiresOpenAIAuth: func() *bool {
				v := true
				return &v
			}(),
		},
		{
			Name:    "reserved-env",
			Family:  FamilyOpenAI,
			Kind:    KindOpenAICompatible,
			Access:  AccessThirdParty,
			Auth:    AuthAPIKey,
			BaseURL: "https://openrouter.ai/api/v1",
			Env: map[string]string{
				"OPENAI_API_KEY": "sk-should-not-be-here",
			},
		},
		{
			Name:    "invalid-env-key",
			Family:  FamilyOpenAI,
			Kind:    KindOpenAICompatible,
			Access:  AccessThirdParty,
			Auth:    AuthAPIKey,
			BaseURL: "https://openrouter.ai/api/v1",
			Env: map[string]string{
				"BAD-KEY": "1",
			},
		},
		{
			Name:    "invalid-env-value",
			Family:  FamilyOpenAI,
			Kind:    KindOpenAICompatible,
			Access:  AccessThirdParty,
			Auth:    AuthAPIKey,
			BaseURL: "https://openrouter.ai/api/v1",
			Env: map[string]string{
				"FOO": "a\nb",
			},
		},
		{
			Name:    "bad-official-base",
			Family:  FamilyOpenAI,
			Kind:    KindOpenAI,
			Access:  AccessOfficial,
			Auth:    AuthAPIKey,
			BaseURL: "https://api.openai.com/v1",
		},
		{
			Name:   "missing-base",
			Family: FamilyClaude,
			Kind:   KindClaude,
			Access: AccessThirdParty,
			Auth:   AuthAPIKey,
		},
		{
			Name:    "wrong-family",
			Family:  FamilyGemini,
			Kind:    KindOpenAICompatible,
			Access:  AccessThirdParty,
			Auth:    AuthAPIKey,
			BaseURL: "https://example.com",
		},
		{
			Name:   "official-openai-compatible",
			Family: FamilyOpenAI,
			Kind:   KindOpenAICompatible,
			Access: AccessOfficial,
			Auth:   AuthAPIKey,
		},
	}
	for _, tc := range tests {
		if err := ValidateTarget(tc); err == nil {
			t.Fatalf("ValidateTarget(%+v) = nil, want error", tc)
		}
	}
}
