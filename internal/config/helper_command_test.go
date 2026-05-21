package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveHelperCommandDoesNotReturnGoBuildArtifact(t *testing.T) {
	command, err := ResolveHelperCommand()
	if err != nil {
		t.Fatalf("ResolveHelperCommand() error = %v", err)
	}
	if command != "" && isEphemeralExecutable(command) {
		t.Fatalf("ResolveHelperCommand() = %q, want non-ephemeral command", command)
	}
}

func TestResolvePathCommandMissing(t *testing.T) {
	if path, ok := resolvePathCommand("agx-command-that-does-not-exist"); ok || path != "" {
		t.Fatalf("resolvePathCommand() = (%q,%v), want empty false", path, ok)
	}
}

func TestIsEphemeralExecutable(t *testing.T) {
	if !isEphemeralExecutable("/tmp/user/.cache/go-build/aa/bb/agx") {
		t.Fatal("isEphemeralExecutable() = false, want true for go-build path")
	}
	if isEphemeralExecutable("/tmp/agx-cache/bin/agx") {
		t.Fatal("isEphemeralExecutable() = true, want false for stable path")
	}
	if strings.Contains("agx", "/go-build/") {
		t.Fatal("unexpected test invariant failure")
	}
}

func TestResolveHelperCommandMatchesCurrentExecutableOnPath(t *testing.T) {
	exe, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("LookPath(go) error = %v", err)
	}
	if _, ok := resolvePathCommand("go"); !ok {
		t.Fatal("resolvePathCommand(go) = false, want true")
	}
	if isEphemeralExecutable(exe) {
		t.Fatalf("test executable path unexpectedly ephemeral: %q", exe)
	}
}

func TestResolvePathCommandFindsTempExecutable(t *testing.T) {
	binDir := t.TempDir()
	scriptPath := filepath.Join(binDir, "agx-helper-test")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	got, ok := resolvePathCommand("agx-helper-test")
	if !ok {
		t.Fatal("resolvePathCommand(temp executable) = false, want true")
	}
	if got != scriptPath {
		t.Fatalf("resolvePathCommand(temp executable) = %q, want %q", got, scriptPath)
	}
}

func TestResolveHelperCommandPrefersAgxWhenPathMatchesCurrentExecutable(t *testing.T) {
	exePath, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable() error = %v", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		exePath = filepath.Clean(exePath)
	}

	binDir := t.TempDir()
	agxPath := filepath.Join(binDir, "agx")
	if err := os.Symlink(exePath, agxPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	got, err := ResolveHelperCommand()
	if err != nil {
		t.Fatalf("ResolveHelperCommand() error = %v", err)
	}
	if got != "agx" {
		t.Fatalf("ResolveHelperCommand() = %q, want agx", got)
	}
}

func TestResolveHelperCommandDoesNotReturnUnexpectedValuesInSubprocess(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestResolveHelperCommandHelper")
	cmd.Env = append(os.Environ(), "AGX_HELPER_RESOLVE_CASE=run")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper process err=%v output=%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got != "" && got != "agx" && !filepath.IsAbs(got) {
		t.Fatalf("ResolveHelperCommand() subprocess output=%q, want empty/agx/absolute path", got)
	}
}

func TestResolveHelperCommandHelper(t *testing.T) {
	if os.Getenv("AGX_HELPER_RESOLVE_CASE") != "run" {
		return
	}
	got, err := ResolveHelperCommand()
	if err != nil {
		t.Fatalf("ResolveHelperCommand() error = %v", err)
	}
	_, _ = os.Stdout.WriteString(got)
}
