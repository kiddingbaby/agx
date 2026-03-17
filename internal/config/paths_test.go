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
	if paths.ConfigDir != wantConfigDir {
		t.Fatalf("ConfigDir = %q, want %q", paths.ConfigDir, wantConfigDir)
	}
	if paths.StorePath != filepath.Join(wantConfigDir, "keys.yaml") {
		t.Fatalf("StorePath = %q", paths.StorePath)
	}
	if paths.SecretPath != filepath.Join(wantConfigDir, "secret") {
		t.Fatalf("SecretPath = %q", paths.SecretPath)
	}
	if paths.ProviderConfigPath != filepath.Join(wantConfigDir, "providers.yaml") {
		t.Fatalf("ProviderConfigPath = %q", paths.ProviderConfigPath)
	}
	if paths.ClaudeSettingsPath != filepath.Join(home, ".claude", "settings.json") {
		t.Fatalf("ClaudeSettingsPath = %q", paths.ClaudeSettingsPath)
	}
	if paths.CodexAuthPath != filepath.Join(home, ".codex", "auth.json") {
		t.Fatalf("CodexAuthPath = %q", paths.CodexAuthPath)
	}
	if paths.CodexConfigPath != filepath.Join(home, ".codex", "config.toml") {
		t.Fatalf("CodexConfigPath = %q", paths.CodexConfigPath)
	}
	if paths.GeminiEnvPath != filepath.Join(home, ".gemini", ".env") {
		t.Fatalf("GeminiEnvPath = %q", paths.GeminiEnvPath)
	}
	if paths.GeminiSettingsPath != filepath.Join(home, ".gemini", "settings.json") {
		t.Fatalf("GeminiSettingsPath = %q", paths.GeminiSettingsPath)
	}
}
