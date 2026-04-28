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
	CodexConfigPath    string
	ClaudeSettingsPath string
	GeminiEnvPath      string
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
		CodexConfigPath:    filepath.Join(homeDir, ".codex", "config.toml"),
		ClaudeSettingsPath: filepath.Join(homeDir, ".claude", "settings.json"),
		GeminiEnvPath:      filepath.Join(homeDir, ".gemini", ".env"),
		BackupsDir:         filepath.Join(configDir, "backups"),
	}, nil
}
