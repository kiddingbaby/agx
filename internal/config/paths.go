package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths contains resolved filesystem paths used by AGX runtime config.
type Paths struct {
	ProfilesDir        string
	StatePath          string
	LockPath           string
	OperationPath      string
	ContextsDir        string
	CodexConfigPath    string
	ClaudeSettingsPath string
	GeminiSettingsPath string
	OpenCodeConfigPath string
	BackupsDir         string
}

// DefaultPaths resolves ~/.config/agx paths.
func DefaultPaths() (Paths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("cannot determine home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "agx")
	return Paths{
		ProfilesDir:        filepath.Join(configDir, "profiles"),
		StatePath:          filepath.Join(configDir, "state.yaml"),
		LockPath:           filepath.Join(configDir, "agx.lock"),
		OperationPath:      filepath.Join(configDir, "ops", "current.yaml"),
		ContextsDir:        filepath.Join(configDir, "contexts"),
		CodexConfigPath:    filepath.Join(homeDir, ".codex", "config.toml"),
		ClaudeSettingsPath: filepath.Join(homeDir, ".claude", "settings.json"),
		GeminiSettingsPath: filepath.Join(homeDir, ".gemini", "settings.json"),
		OpenCodeConfigPath: filepath.Join(homeDir, ".config", "opencode", "opencode.json"),
		BackupsDir:         filepath.Join(configDir, "backups"),
	}, nil
}
