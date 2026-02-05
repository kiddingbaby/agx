package tui

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{
			name:  "shorter than max",
			input: "hello",
			max:   10,
			want:  "hello",
		},
		{
			name:  "equal to max",
			input: "hello",
			max:   5,
			want:  "hello",
		},
		{
			name:  "longer than max",
			input: "hello world",
			max:   8,
			want:  "hello...",
		},
		{
			name:  "max less than 4",
			input: "hello",
			max:   3,
			want:  "hel",
		},
		{
			name:  "empty string",
			input: "",
			max:   10,
			want:  "",
		},
		{
			name:  "unicode bytes",
			input: "你好世界",
			max:   12, // 4 chars * 3 bytes = 12 bytes
			want:  "你好世界",
		},
		{
			name:  "unicode truncated",
			input: "你好世界",
			max:   6, // 2 chars * 3 bytes = 6 bytes, but len("你好世界") = 12 > 6
			want:  "你...", // truncate works on bytes: s[:3] = "你"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestGetCwd(t *testing.T) {
	// GetCwd should return a non-empty path
	cwd := GetCwd()
	if cwd == "" {
		t.Error("GetCwd() returned empty string")
	}
}

func TestSplitTags(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b, c", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{"", nil},
		{"  spaced  , values  ", []string{"spaced", "values"}},
	}

	for _, tt := range tests {
		result := splitTags(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("splitTags(%q) = %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("splitTags(%q)[%d] = %v, want %v", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestDefaultAgents(t *testing.T) {
	agents := DefaultAgents()

	if len(agents) != 3 {
		t.Errorf("DefaultAgents() returned %d agents, want 3", len(agents))
	}

	expected := []struct {
		name     string
		command  string
		envVar   string
		provider string
	}{
		{"claude-code", "claude", "ANTHROPIC_API_KEY", "claude"},
		{"codex-cli", "codex", "OPENAI_API_KEY", "openai"},
		{"gemini-cli", "gemini", "GOOGLE_API_KEY", "gemini"},
	}

	for i, exp := range expected {
		if agents[i].Name != exp.name {
			t.Errorf("agents[%d].Name = %v, want %v", i, agents[i].Name, exp.name)
		}
		if agents[i].Command != exp.command {
			t.Errorf("agents[%d].Command = %v, want %v", i, agents[i].Command, exp.command)
		}
		if agents[i].EnvVar != exp.envVar {
			t.Errorf("agents[%d].EnvVar = %v, want %v", i, agents[i].EnvVar, exp.envVar)
		}
		if agents[i].Provider != exp.provider {
			t.Errorf("agents[%d].Provider = %v, want %v", i, agents[i].Provider, exp.provider)
		}
	}
}
