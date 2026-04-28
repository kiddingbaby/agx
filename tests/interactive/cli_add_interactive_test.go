//go:build interactive

package interactive

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	expect "github.com/Netflix/go-expect"
)

func TestAddProfileInteractivePTY(t *testing.T) {
	home := t.TempDir()

	console, err := expect.NewConsole(expect.WithDefaultTimeout(3 * time.Second))
	if err != nil {
		t.Fatalf("NewConsole() error = %v", err)
	}
	defer console.Close()

	cmd := exec.Command(binaryPath(t), "add")
	cmd.Env = append(os.Environ(), "HOME="+home)
	cmd.Stdin = console.Tty()
	cmd.Stdout = console.Tty()
	cmd.Stderr = console.Tty()

	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	expectString(t, console, "Relay name: ")
	sendLine(t, console, "relay-pty")
	expectString(t, console, "Base URL: ")
	sendLine(t, console, "https://relay-pty.example/v1")
	expectString(t, console, "API key: ")
	sendLine(t, console, "sk-pty")
	expectString(t, console, "Added relay: relay-pty")

	if err := waitCommand(cmd, 3*time.Second); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	show := exec.Command(binaryPath(t), "show", "relay-pty")
	show.Env = append(os.Environ(), "HOME="+home)
	output, err := show.CombinedOutput()
	if err != nil {
		t.Fatalf("show error = %v\n%s", err, output)
	}
	got := string(output)
	if !strings.Contains(got, "base_url=https://relay-pty.example/v1") || !strings.Contains(got, "api_key=sk-pty") {
		t.Fatalf("show output=%q want persisted interactive add values", got)
	}
}

func TestEditProfileInteractivePTY(t *testing.T) {
	home := t.TempDir()

	add := exec.Command(binaryPath(t), "add", "relay-a", "--base-url", "https://relay.example/v1", "--api-key", "sk-a")
	add.Env = append(os.Environ(), "HOME="+home)
	if output, err := add.CombinedOutput(); err != nil {
		t.Fatalf("seed add error = %v\n%s", err, output)
	}

	console, err := expect.NewConsole(expect.WithDefaultTimeout(3 * time.Second))
	if err != nil {
		t.Fatalf("NewConsole() error = %v", err)
	}
	defer console.Close()

	cmd := exec.Command(binaryPath(t), "edit", "relay-a")
	cmd.Env = append(os.Environ(), "HOME="+home)
	cmd.Stdin = console.Tty()
	cmd.Stdout = console.Tty()
	cmd.Stderr = console.Tty()

	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	expectString(t, console, "Current relay: relay-a")
	expectString(t, console, "Edit [1 url, 2 key, Enter done]: ")
	sendLine(t, console, "2")
	expectString(t, console, "API key [keep current]: ")
	sendLine(t, console, "sk-rotated")
	expectString(t, console, "Edit more [1 url, 2 key, Enter done]: ")
	sendLine(t, console, "")
	expectString(t, console, "Edited relay: relay-a")

	if err := waitCommand(cmd, 3*time.Second); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	show := exec.Command(binaryPath(t), "show", "relay-a")
	show.Env = append(os.Environ(), "HOME="+home)
	output, err := show.CombinedOutput()
	if err != nil {
		t.Fatalf("show error = %v\n%s", err, output)
	}
	got := string(output)
	if !strings.Contains(got, "base_url=https://relay.example/v1") || !strings.Contains(got, "api_key=sk-rotated") {
		t.Fatalf("show output=%q want persisted interactive edit values", got)
	}
}

func binaryPath(t *testing.T) string {
	t.Helper()

	if cacheDir := os.Getenv("AGX_CACHE_DIR"); cacheDir != "" {
		return filepath.Join(cacheDir, "bin", "agx")
	}
	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		return filepath.Join(cacheHome, "agx", "bin", "agx")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	return filepath.Join(home, ".cache", "agx", "bin", "agx")
}

func expectString(t *testing.T, console *expect.Console, value string) {
	t.Helper()

	if _, err := console.ExpectString(value); err != nil {
		t.Fatalf("ExpectString(%q) error = %v", value, err)
	}
}

func sendLine(t *testing.T, console *expect.Console, value string) {
	t.Helper()

	if _, err := console.SendLine(value); err != nil {
		t.Fatalf("SendLine(%q) error = %v", value, err)
	}
}

func waitCommand(cmd *exec.Cmd, timeout time.Duration) error {
	done := make(chan error, 1)

	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return <-done
	}
}
