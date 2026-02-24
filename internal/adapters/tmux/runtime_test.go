package tmux

import (
	"os/exec"
	"testing"

	domainsession "github.com/kiddingbaby/agx/internal/domain/session"
)

func TestNewRuntime(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}

	r, err := NewRuntime()
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	if r == nil {
		t.Fatal("NewRuntime() returned nil")
	}
}

func TestSessionConfig(t *testing.T) {
	cfg := domainsession.SessionConfig{
		Agent: "ai-claude-code",
		Dir:   "/tmp",
		EnvVars: map[string]string{
			"ANTHROPIC_API_KEY": "test-key",
		},
	}

	if cfg.Agent != "ai-claude-code" {
		t.Errorf("Agent = %v, want ai-claude-code", cfg.Agent)
	}
	if cfg.Dir != "/tmp" {
		t.Errorf("Dir = %v, want /tmp", cfg.Dir)
	}
	if cfg.EnvVars["ANTHROPIC_API_KEY"] != "test-key" {
		t.Errorf("EnvVars[ANTHROPIC_API_KEY] = %v, want test-key", cfg.EnvVars["ANTHROPIC_API_KEY"])
	}
}

func TestListSessions(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}

	r, err := NewRuntime()
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}

	if _, err := r.ListSessions(); err != nil {
		t.Errorf("ListSessions() error = %v", err)
	}
}
