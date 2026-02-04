package session

import (
	"os/exec"
	"testing"
)

func TestNewOrchestrator(t *testing.T) {
	// Skip if tmux is not installed
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}

	orch, err := NewOrchestrator()
	if err != nil {
		t.Fatalf("NewOrchestrator() error = %v", err)
	}
	if orch == nil {
		t.Fatal("NewOrchestrator() returned nil")
	}
	if orch.tmuxPath == "" {
		t.Error("tmuxPath is empty")
	}
}

func TestSessionConfig(t *testing.T) {
	cfg := SessionConfig{
		Agent: "claude-code",
		Dir:   "/tmp",
		EnvVars: map[string]string{
			"ANTHROPIC_API_KEY": "test-key",
		},
	}

	if cfg.Agent != "claude-code" {
		t.Errorf("Agent = %v, want claude-code", cfg.Agent)
	}
	if cfg.Dir != "/tmp" {
		t.Errorf("Dir = %v, want /tmp", cfg.Dir)
	}
	if cfg.EnvVars["ANTHROPIC_API_KEY"] != "test-key" {
		t.Errorf("EnvVars[ANTHROPIC_API_KEY] = %v, want test-key", cfg.EnvVars["ANTHROPIC_API_KEY"])
	}
}

func TestListSessions(t *testing.T) {
	// Skip if tmux is not installed
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}

	orch, err := NewOrchestrator()
	if err != nil {
		t.Fatalf("NewOrchestrator() error = %v", err)
	}

	// ListSessions should not error even if no sessions exist
	sessions, err := orch.ListSessions()
	if err != nil {
		t.Errorf("ListSessions() error = %v", err)
	}
	// sessions can be nil or empty, both are valid
	_ = sessions
}

func TestEscapeForShell(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple string",
			input: "hello",
			want:  "$'hello'",
		},
		{
			name:  "single quote",
			input: "it's",
			want:  "$'it\\'s'",
		},
		{
			name:  "backslash",
			input: "path\\to\\file",
			want:  "$'path\\\\to\\\\file'",
		},
		{
			name:  "newline",
			input: "line1\nline2",
			want:  "$'line1\\nline2'",
		},
		{
			name:  "tab",
			input: "col1\tcol2",
			want:  "$'col1\\tcol2'",
		},
		{
			name:  "carriage return",
			input: "line\r",
			want:  "$'line\\r'",
		},
		{
			name:  "dollar sign preserved",
			input: "$HOME",
			want:  "$'$HOME'",
		},
		{
			name:  "backtick preserved",
			input: "`cmd`",
			want:  "$'`cmd`'",
		},
		{
			name:  "api key with special chars",
			input: "sk-ant-api03-xxx'yyy$zzz",
			want:  "$'sk-ant-api03-xxx\\'yyy$zzz'",
		},
		{
			name:  "empty string",
			input: "",
			want:  "$''",
		},
		// Additional edge cases
		{
			name:  "multiple single quotes",
			input: "it's Bob's",
			want:  "$'it\\'s Bob\\'s'",
		},
		{
			name:  "mixed special chars",
			input: "key'with\\special\nchars",
			want:  "$'key\\'with\\\\special\\nchars'",
		},
		{
			name:  "unicode preserved",
			input: "hello 世界",
			want:  "$'hello 世界'",
		},
		{
			name:  "double quote preserved",
			input: `say "hello"`,
			want:  `$'say "hello"'`,
		},
		{
			name:  "real API key format",
			input: "sk-ant-api03-abcdefghijklmnopqrstuvwxyz",
			want:  "$'sk-ant-api03-abcdefghijklmnopqrstuvwxyz'",
		},
		{
			name:  "null byte not expected but handled",
			input: "before\x00after",
			want:  "$'before\x00after'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeForShell(tt.input)
			if got != tt.want {
				t.Errorf("escapeForShell(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
