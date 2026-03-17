package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths contains resolved filesystem paths used by AGX runtime config.
type Paths struct {
	HomeDir            string
	ConfigDir          string
	StorePath          string
	SecretPath         string
	ProviderConfigPath string

	ClaudeDir          string
	ClaudeSettingsPath string

	CodexDir        string
	CodexAuthPath   string
	CodexConfigPath string

	GeminiDir          string
	GeminiEnvPath      string
	GeminiSettingsPath string
}

// DefaultPaths resolves ~/.config/agx paths.
func DefaultPaths() (Paths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("cannot determine home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "agx")
	claudeDir := filepath.Join(homeDir, ".claude")
	codexDir := filepath.Join(homeDir, ".codex")
	geminiDir := filepath.Join(homeDir, ".gemini")
	return Paths{
		HomeDir:            homeDir,
		ConfigDir:          configDir,
		StorePath:          filepath.Join(configDir, "keys.yaml"),
		SecretPath:         filepath.Join(configDir, "secret"),
		ProviderConfigPath: filepath.Join(configDir, "providers.yaml"),
		ClaudeDir:          claudeDir,
		ClaudeSettingsPath: filepath.Join(claudeDir, "settings.json"),
		CodexDir:           codexDir,
		CodexAuthPath:      filepath.Join(codexDir, "auth.json"),
		CodexConfigPath:    filepath.Join(codexDir, "config.toml"),
		GeminiDir:          geminiDir,
		GeminiEnvPath:      filepath.Join(geminiDir, ".env"),
		GeminiSettingsPath: filepath.Join(geminiDir, "settings.json"),
	}, nil
}
