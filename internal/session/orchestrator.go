package session

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

// escapeForShell uses $'...' syntax for complete escaping
func escapeForShell(value string) string {
	var b strings.Builder
	b.WriteString("$'")
	for _, r := range value {
		switch r {
		case '\'':
			b.WriteString("\\'")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			b.WriteRune(r)
		}
	}
	b.WriteString("'")
	return b.String()
}

// Launch creates or attaches to a tmux session
func (o *Orchestrator) Launch(cfg SessionConfig) error {
	// Validate directory exists
	if _, err := os.Stat(cfg.Dir); err != nil {
		return fmt.Errorf("directory does not exist: %s", cfg.Dir)
	}

	sessionName := fmt.Sprintf("ai-%s", cfg.Agent)
	windowName := filepath.Base(cfg.Dir)

	// Build environment export string with proper escaping
	// Sort keys for deterministic output
	var envExports []string
	keys := make([]string, 0, len(cfg.EnvVars))
	for k := range cfg.EnvVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := cfg.EnvVars[k]
		envExports = append(envExports, fmt.Sprintf("export %s=%s", k, escapeForShell(v)))
	}
	envStr := strings.Join(envExports, " && ")

	// Build the command to run
	shellCmd := cfg.Command
	if envStr != "" {
		shellCmd = envStr + " && " + cfg.Command
	}

	// Check if session exists
	// Note: hasSession + create has potential race condition but is acceptable
	// for this use case since sessions are user-initiated
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
	return o.Attach(sessionName)
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

// Attach attaches to an existing session
func (o *Orchestrator) Attach(sessionName string) error {
	// Check if already inside tmux
	if os.Getenv("TMUX") != "" {
		// Use switch-client instead of attach
		cmd := exec.Command(o.tmuxPath, "switch-client", "-t", sessionName)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	cmd := exec.Command(o.tmuxPath, "attach", "-t", sessionName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SessionInfo holds information about an active session
type SessionInfo struct {
	Name       string
	Windows    int
	CreatedAt  string
	Attached   bool
}

// ListSessions returns all active AI sessions
func (o *Orchestrator) ListSessions() ([]SessionInfo, error) {
	cmd := exec.Command(o.tmuxPath, "list-sessions", "-F", "#{session_name}\t#{session_windows}\t#{session_created_string}\t#{session_attached}")
	output, err := cmd.Output()
	if err != nil {
		// Check if this is just "no sessions" (exit code 1)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil // No sessions is not an error
		}
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	var sessions []SessionInfo
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 1 || !strings.HasPrefix(parts[0], "ai-") {
			continue
		}
		info := SessionInfo{Name: parts[0]}
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d", &info.Windows)
		}
		if len(parts) >= 3 {
			info.CreatedAt = parts[2]
		}
		if len(parts) >= 4 {
			info.Attached = parts[3] == "1"
		}
		sessions = append(sessions, info)
	}
	return sessions, nil
}

// KillSession terminates a session
func (o *Orchestrator) KillSession(name string) error {
	return o.run("kill-session", "-t", name)
}
