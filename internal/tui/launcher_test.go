package tui

import "testing"

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

func TestNewLauncher(t *testing.T) {
	launcher := NewLauncher()

	if launcher == nil {
		t.Fatal("NewLauncher() returned nil")
	}

	if launcher.GetItemCount() != 3 {
		t.Errorf("Launcher has %d items, want 3", launcher.GetItemCount())
	}
}

func TestLauncherCallbacks(t *testing.T) {
	launcher := NewLauncher()

	var selectCalled bool
	var cancelCalled bool

	launcher.SetOnSelect(func(agent Agent) {
		selectCalled = true
	})

	launcher.SetOnCancel(func() {
		cancelCalled = true
	})

	// Verify callbacks are set
	if launcher.onSelect == nil {
		t.Error("onSelect callback not set")
	}
	if launcher.onCancel == nil {
		t.Error("onCancel callback not set")
	}

	// Test cancel callback
	launcher.onCancel()
	if !cancelCalled {
		t.Error("onCancel callback not called")
	}

	// Test select callback
	launcher.onSelect(DefaultAgents()[0])
	if !selectCalled {
		t.Error("onSelect callback not called")
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
		{"a,,b", []string{"a", "", "b"}},
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
