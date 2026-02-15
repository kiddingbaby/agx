package tmux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	domainsession "github.com/kiddingbaby/agx/internal/domain/session"
)

// Runtime implements ports.SessionRuntime with tmux.
type Runtime struct {
	tmuxPath string
}

func NewRuntime() (*Runtime, error) {
	path, err := exec.LookPath("tmux")
	if err != nil {
		return nil, errors.New("tmux not found in PATH")
	}
	return &Runtime{tmuxPath: path}, nil
}

// Launch creates or attaches to a tmux session.
// Secrets are injected via tmux session environment, not shell command exports.
func (r *Runtime) Launch(cfg domainsession.SessionConfig) error {
	if cfg.Agent == "" {
		return errors.New("session name is required")
	}
	if _, err := os.Stat(cfg.Dir); err != nil {
		return fmt.Errorf("directory does not exist: %s", cfg.Dir)
	}

	sessionName := cfg.Agent
	windowName := filepath.Base(cfg.Dir)

	if r.hasSession(sessionName) {
		if err := r.setSessionEnv(sessionName, cfg.EnvVars); err != nil {
			return fmt.Errorf("failed to set session env: %w", err)
		}
		args := []string{
			"new-window",
			"-t", sessionName,
			"-n", windowName,
			"-c", cfg.Dir,
			cfg.Command,
		}
		if err := r.run(args...); err != nil {
			return fmt.Errorf("failed to create window: %w", err)
		}
	} else {
		args := []string{
			"new-session",
			"-d",
			"-s", sessionName,
			"-n", windowName,
			"-c", cfg.Dir,
		}
		if err := r.run(args...); err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
		if err := r.setSessionEnv(sessionName, cfg.EnvVars); err != nil {
			return fmt.Errorf("failed to set session env: %w", err)
		}
		paneTarget := fmt.Sprintf("%s:%s.0", sessionName, windowName)
		if err := r.run("respawn-pane", "-k", "-t", paneTarget, "-c", cfg.Dir, cfg.Command); err != nil {
			return fmt.Errorf("failed to start command in pane: %w", err)
		}
	}

	return r.Attach(sessionName)
}

func (r *Runtime) setSessionEnv(sessionName string, env map[string]string) error {
	if len(env) == 0 {
		return nil
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if err := r.run("set-environment", "-t", sessionName, k, env[k]); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runtime) hasSession(name string) bool {
	cmd := exec.Command(r.tmuxPath, "has-session", "-t", name)
	return cmd.Run() == nil
}

func (r *Runtime) run(args ...string) error {
	cmd := exec.Command(r.tmuxPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *Runtime) Attach(sessionName string) error {
	if os.Getenv("TMUX") != "" {
		cmd := exec.Command(r.tmuxPath, "switch-client", "-t", sessionName)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	cmd := exec.Command(r.tmuxPath, "attach", "-t", sessionName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *Runtime) ListSessions() ([]domainsession.SessionInfo, error) {
	cmd := exec.Command(r.tmuxPath, "list-sessions", "-F", "#{session_name}\t#{session_windows}\t#{session_created_string}\t#{session_attached}")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	var sessions []domainsession.SessionInfo
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 1 {
			continue
		}

		info := domainsession.SessionInfo{Name: parts[0]}
		if len(parts) >= 2 {
			windows, convErr := strconv.Atoi(parts[1])
			if convErr == nil {
				info.Windows = windows
			}
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

func (r *Runtime) KillSession(name string) error {
	return r.run("kill-session", "-t", name)
}
