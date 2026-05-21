package profile

import "testing"

func TestSupportedAgents(t *testing.T) {
	agents := SupportedAgents()
	if len(agents) != 4 {
		t.Fatalf("len(agents) = %d, want 4", len(agents))
	}

	agents[0] = "mutated"
	fresh := SupportedAgents()
	if fresh[0] != AgentCodex {
		t.Fatalf("SupportedAgents should return a copy")
	}
}

func TestParseAgent(t *testing.T) {
	cases := []struct {
		input string
		want  Agent
		ok    bool
	}{
		{input: " CODEX ", want: AgentCodex, ok: true},
		{input: "claude", want: AgentClaude, ok: true},
		{input: "gemini", want: AgentGemini, ok: true},
		{input: "opencode", want: AgentOpenCode, ok: true},
		{input: "openai", want: "", ok: false},
	}

	for _, tc := range cases {
		got, ok := ParseAgent(tc.input)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("ParseAgent(%q) = (%q,%v), want (%q,%v)", tc.input, got, ok, tc.want, tc.ok)
		}
	}
}

func TestNormalizeProfileName(t *testing.T) {
	if got := NormalizeProfileName(" Relay-Prod "); got != "relay-prod" {
		t.Fatalf("NormalizeProfileName() = %q, want relay-prod", got)
	}
}

func TestNormalizeTargetName(t *testing.T) {
	if got := NormalizeTargetName(" Work "); got != "work" {
		t.Fatalf("NormalizeTargetName() = %q, want work", got)
	}
}
