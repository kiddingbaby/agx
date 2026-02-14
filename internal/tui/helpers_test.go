package tui

import (
	"testing"
	"time"

	"github.com/kiddingbaby/agx/internal/key"
)

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
			name:  "unicode fits in rune count",
			input: "你好世界",
			max:   12,
			want:  "你好世界",
		},
		{
			name:  "unicode fits exactly",
			input: "你好世界",
			max:   4,
			want:  "你好世界",
		},
		{
			name:  "unicode truncated",
			input: "你好世界测试",
			max:   5,
			want:  "你好...",
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

func TestDisplayDate(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	if got := displayDate(key.Key{CreatedAt: created}); !got.Equal(created) {
		t.Fatalf("displayDate(created only) = %v, want %v", got, created)
	}
	if got := displayDate(key.Key{CreatedAt: created, UpdatedAt: updated}); !got.Equal(updated) {
		t.Fatalf("displayDate(with updated) = %v, want %v", got, updated)
	}
}
