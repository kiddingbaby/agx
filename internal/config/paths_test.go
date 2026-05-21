package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	paths, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths() error = %v", err)
	}

	wantConfigDir := filepath.Join(home, ".config", "agx")
	if paths.ProfilesDir != filepath.Join(wantConfigDir, "profiles") {
		t.Fatalf("ProfilesDir = %q, want %q", paths.ProfilesDir, filepath.Join(wantConfigDir, "profiles"))
	}
	if paths.StatePath != filepath.Join(wantConfigDir, "state.yaml") {
		t.Fatalf("StatePath = %q, want %q", paths.StatePath, filepath.Join(wantConfigDir, "state.yaml"))
	}
	if paths.LockPath != filepath.Join(wantConfigDir, "agx.lock") {
		t.Fatalf("LockPath = %q, want %q", paths.LockPath, filepath.Join(wantConfigDir, "agx.lock"))
	}
	if paths.OperationPath != filepath.Join(wantConfigDir, "ops", "current.yaml") {
		t.Fatalf("OperationPath = %q, want %q", paths.OperationPath, filepath.Join(wantConfigDir, "ops", "current.yaml"))
	}
	if paths.ContextsDir != filepath.Join(wantConfigDir, "contexts") {
		t.Fatalf("ContextsDir = %q, want %q", paths.ContextsDir, filepath.Join(wantConfigDir, "contexts"))
	}
	if paths.CodexConfigPath != filepath.Join(home, ".codex", "config.toml") {
		t.Fatalf("CodexConfigPath = %q, want %q", paths.CodexConfigPath, filepath.Join(home, ".codex", "config.toml"))
	}
	if paths.ClaudeSettingsPath != filepath.Join(home, ".claude", "settings.json") {
		t.Fatalf("ClaudeSettingsPath = %q, want %q", paths.ClaudeSettingsPath, filepath.Join(home, ".claude", "settings.json"))
	}
	if paths.GeminiSettingsPath != filepath.Join(home, ".gemini", "settings.json") {
		t.Fatalf("GeminiSettingsPath = %q, want %q", paths.GeminiSettingsPath, filepath.Join(home, ".gemini", "settings.json"))
	}
	if paths.BackupsDir != filepath.Join(wantConfigDir, "backups") {
		t.Fatalf("BackupsDir = %q, want %q", paths.BackupsDir, filepath.Join(wantConfigDir, "backups"))
	}
}

func TestDefaultPathsUsesUpdatedHomeEachCall(t *testing.T) {
	homeA := t.TempDir()
	t.Setenv("HOME", homeA)
	pathsA, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths(homeA) error = %v", err)
	}

	homeB := t.TempDir()
	t.Setenv("HOME", homeB)
	pathsB, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths(homeB) error = %v", err)
	}

	if pathsA.CodexConfigPath == pathsB.CodexConfigPath {
		t.Fatalf("CodexConfigPath did not change across HOME values: %q", pathsA.CodexConfigPath)
	}
	if pathsB.CodexConfigPath != filepath.Join(homeB, ".codex", "config.toml") {
		t.Fatalf("CodexConfigPath = %q, want %q", pathsB.CodexConfigPath, filepath.Join(homeB, ".codex", "config.toml"))
	}
}
