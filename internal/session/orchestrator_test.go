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
		Agent:   "claude-code",
		Dir:     "/tmp",
		Command: "claude",
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
