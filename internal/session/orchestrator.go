package session

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Orchestrator manages tmux sessions
type Orchestrator struct {
	tmuxPath string
}

// NewOrchestrator creates a new session orchestrator
func NewOrchestrator() (*Orchestrator, error) {
	path, err := exec.LookPath("tmux")
	if err != nil {
		return nil, errors.New("tmux not found in PATH")
	}
	return &Orchestrator{tmuxPath: path}, nil
}

// SessionConfig holds configuration for creating a session
type SessionConfig struct {
	Agent   string
	Dir     string
	Command string
	EnvVars map[string]string
}

// Launch creates or attaches to a tmux session
func (o *Orchestrator) Launch(cfg SessionConfig) error {
	sessionName := fmt.Sprintf("ai-%s", cfg.Agent)
	windowName := filepath.Base(cfg.Dir)

	// Build environment export string
	var envExports []string
	for k, v := range cfg.EnvVars {
		envExports = append(envExports, fmt.Sprintf("export %s='%s'", k, v))
	}
	envStr := strings.Join(envExports, " && ")

	// Build the command to run
	shellCmd := cfg.Command
	if envStr != "" {
		shellCmd = envStr + " && " + cfg.Command
	}

	// Check if session exists
	if o.hasSession(sessionName) {
		// Create new window in existing session
		args := []string{
			"new-window",
			"-t", sessionName,
			"-n", windowName,
			"-c", cfg.Dir,
			shellCmd,
		}
		if err := o.run(args...); err != nil {
			return fmt.Errorf("failed to create window: %w", err)
		}
	} else {
		// Create new session
		args := []string{
			"new-session",
			"-d",
			"-s", sessionName,
			"-n", windowName,
			"-c", cfg.Dir,
			shellCmd,
		}
		if err := o.run(args...); err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
	}

	// Attach to session
	return o.attach(sessionName)
}

func (o *Orchestrator) hasSession(name string) bool {
	cmd := exec.Command(o.tmuxPath, "has-session", "-t", name)
	return cmd.Run() == nil
}

func (o *Orchestrator) run(args ...string) error {
	cmd := exec.Command(o.tmuxPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (o *Orchestrator) attach(sessionName string) error {
	cmd := exec.Command(o.tmuxPath, "attach", "-t", sessionName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ListSessions returns all active AI sessions
func (o *Orchestrator) ListSessions() ([]string, error) {
	cmd := exec.Command(o.tmuxPath, "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		return nil, nil // No sessions
	}

	var sessions []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "ai-") {
			sessions = append(sessions, line)
		}
	}
	return sessions, nil
}

// KillSession terminates a session
func (o *Orchestrator) KillSession(name string) error {
	return o.run("kill-session", "-t", name)
}
