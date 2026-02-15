package session

import "testing"

func TestSessionName(t *testing.T) {
	if got := SessionName("claude-code"); got != "ai-claude-code" {
		t.Fatalf("SessionName() = %q, want %q", got, "ai-claude-code")
	}
}

func TestNormalizeSessionName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "codex-cli", want: "ai-codex-cli"},
		{input: "ai-gemini-cli", want: "ai-gemini-cli"},
	}

	for _, tt := range tests {
		if got := NormalizeSessionName(tt.input); got != tt.want {
			t.Fatalf("NormalizeSessionName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsManagedSessionName(t *testing.T) {
	if !IsManagedSessionName("ai-claude-code") {
		t.Fatal("expected ai-claude-code to be managed")
	}
	if IsManagedSessionName("workspace") {
		t.Fatal("expected workspace to be unmanaged")
	}
}
